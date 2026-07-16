// Package player wraps gotracker/playback: it loads a tracker module from
// bytes, renders it tick by tick on a background goroutine, and exposes
// (a) a pull-based PCM stream for ebiten's audio player and (b) sample-
// position-keyed state — order/row/BPM, per-channel VU, and oscilloscope
// taps — so the UI can show exactly what is audible.
package player

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gotracker/playback/format"
	"github.com/gotracker/playback/index"
	"github.com/gotracker/playback/mixing"
	"github.com/gotracker/playback/mixing/sampling"
	"github.com/gotracker/playback/mixing/volume"
	"github.com/gotracker/playback/output"
	"github.com/gotracker/playback/player/feature"
	"github.com/gotracker/playback/player/machine"
	"github.com/gotracker/playback/player/machine/settings"
	"github.com/gotracker/playback/player/render"
	"github.com/gotracker/playback/player/sampler"
	"github.com/gotracker/playback/song"
)

const (
	SampleRate    = 44100
	Channels      = 2
	BytesPerFrame = 4                              // s16le stereo
	ringCapacity  = SampleRate / 4 * BytesPerFrame // ~250ms of backpressure
	scopeCapacity = SampleRate                     // 1s of scope history per channel
)

// SongInfo is the immutable metadata captured at load.
type SongInfo struct {
	Name         string
	Format       string
	NumChannels  int
	NumOrders    int
	InitialBPM   int
	InitialSpeed int
	Orders       []int              // order position -> pattern index
	PatternText  map[int][][]string // pattern index -> rows -> per-channel cells
}

// orderSeeker is the richer surface the concrete machine implements beyond
// MachineTicker; feature-detected at load so seek degrades gracefully.
type orderSeeker interface {
	SetOrder(index.Order) error
}

type Player struct {
	Info SongInfo

	mach  machine.MachineTicker
	out   *sampler.Sampler
	mixer mixing.Mixer
	seek  orderSeeker // nil when unsupported

	ring   *pcmRing
	taps   []*scopeTap
	states stateQueue

	mu       sync.Mutex   // guards the render loop's machine access
	rendered int64        // samples rendered so far
	gain     atomic.Int64 // milli-gain (1000 = 1.0)
	paused   atomic.Bool
	finished atomic.Bool
	resume   chan struct{}
	stop     chan struct{}
	stopOnce sync.Once

	curOrder atomic.Int64 // last order seen by onGenerate (for SeekOrder deltas)

	// latencyOffset is subtracted from the ring playhead to approximate the
	// audio device's own buffering; tuned by eye, nudged via debug keys.
	LatencyOffset int64
}

// Load parses a module (MOD/S3M/XM/IT — auto-detected) and prepares a Player.
func Load(data []byte) (*Player, error) {
	features := []feature.Feature{
		feature.UseNativeSampleFormat(true),
		feature.IgnoreUnknownEffect{Enabled: true},
		feature.SongLoop{Count: 0},
	}
	songData, songFormat, err := format.LoadFromReader("", bytes.NewReader(data), features...)
	if err != nil {
		return nil, fmt.Errorf("not a playable module: %w", err)
	}
	var us settings.UserSettings
	if err := songFormat.ConvertFeaturesToSettings(&us, features); err != nil {
		return nil, err
	}
	mach, err := machine.NewMachine(songData, us)
	if err != nil {
		return nil, err
	}

	p := &Player{
		mach:          mach,
		mixer:         mixing.Mixer{Channels: Channels},
		ring:          newPCMRing(ringCapacity),
		resume:        make(chan struct{}, 1),
		stop:          make(chan struct{}),
		LatencyOffset: SampleRate / 10, // ~100ms; matches the audio player buffer
	}
	p.gain.Store(1000)
	p.seek, _ = mach.(orderSeeker)
	p.Info = buildInfo(songData, data)
	p.taps = make([]*scopeTap, p.Info.NumChannels)
	for i := range p.taps {
		p.taps[i] = newScopeTap(scopeCapacity)
	}
	p.out = sampler.NewSampler(SampleRate, Channels, 1.0, p.onGenerate)
	return p, nil
}

func buildInfo(sd song.Data, raw []byte) SongInfo {
	info := SongInfo{
		Name:         strings.TrimRight(sd.GetName(), "\x00 "),
		Format:       sniffFormat(raw),
		NumChannels:  sd.GetNumChannels(),
		InitialBPM:   sd.GetInitialBPM(),
		InitialSpeed: sd.GetInitialTempo(),
		PatternText:  map[int][][]string{},
	}
	orders := sd.GetOrderList()
	info.NumOrders = len(orders)
	for _, pat := range orders {
		info.Orders = append(info.Orders, int(pat))
	}
	for _, patIdx := range info.Orders {
		if _, done := info.PatternText[patIdx]; done {
			continue
		}
		pat, err := sd.GetPattern(index.Pattern(patIdx))
		if err != nil {
			continue
		}
		rows := make([][]string, pat.NumRows())
		for r := range pat.NumRows() {
			row := pat.GetRow(index.Row(r))
			text := sd.GetRowRenderStringer(row, info.NumChannels, false).String()
			rows[r] = splitRowText(text)
		}
		info.PatternText[patIdx] = rows
	}
	return info
}

// splitRowText turns "|C-5 01 .. ...|--- .. .. ...|" into per-channel cells.
func splitRowText(s string) []string {
	parts := strings.Split(s, "|")
	cells := parts[:0]
	for _, c := range parts {
		if strings.TrimSpace(c) != "" || c != "" {
			if c == "" {
				continue
			}
			cells = append(cells, c)
		}
	}
	return cells
}

// sniffFormat identifies the container by magic for the UI badge.
func sniffFormat(b []byte) string {
	switch {
	case len(b) >= 4 && string(b[:4]) == "IMPM":
		return "IT"
	case len(b) >= 17 && string(b[:17]) == "Extended Module: ":
		return "XM"
	case len(b) >= 48 && string(b[44:48]) == "SCRM":
		return "S3M"
	case len(b) >= 1084:
		return "MOD"
	default:
		return "MOD"
	}
}

// Start launches the render goroutine.
func (p *Player) Start() {
	go p.loop()
}

func (p *Player) loop() {
	defer p.ring.Close()
	for {
		select {
		case <-p.stop:
			return
		default:
		}
		if p.paused.Load() {
			select {
			case <-p.resume:
			case <-p.stop:
				return
			}
			continue
		}
		p.mu.Lock()
		err := p.mach.Advance()
		if err == nil {
			err = p.mach.Render(p.out)
		}
		p.mu.Unlock()
		if err != nil {
			if !errors.Is(err, song.ErrStopSong) {
				// Render errors on hostile files end playback gracefully.
				_ = err
			}
			p.finished.Store(true)
			return
		}
	}
}

// onGenerate runs synchronously inside Render for every tick (~20ms of audio).
func (p *Player) onGenerate(premix *output.PremixData) {
	start := p.rendered

	ev := stateEvent{startSample: start, bpm: deriveBPM(premix.SamplesLen)}
	if rr, ok := premix.Userdata.(*render.RowRender); ok && rr != nil {
		ev.order, ev.row, ev.tick = rr.Order, rr.Row, rr.Tick
		p.curOrder.Store(int64(rr.Order))
	}
	ev.vu = make([]float32, p.Info.NumChannels)
	for ch := 0; ch < p.Info.NumChannels && ch < len(premix.Data); ch++ {
		mono := foldChannel(premix.Data[ch], premix.SamplesLen)
		p.taps[ch].Write(mono)
		ev.vu[ch] = peak(mono)
	}
	p.states.Push(ev)

	gain := volume.Volume(float64(p.gain.Load()) / 1000)
	pcm := p.mixer.Flatten(premix.SamplesLen, premix.Data, premix.MixerVolume*gain, sampling.Format16BitLESigned)
	p.ring.Write(pcm)
	p.rendered += int64(premix.SamplesLen)
}

// deriveBPM inverts SamplesLen = SampleRate * 2.5/BPM (classic tempo mode).
func deriveBPM(samplesLen int) int {
	if samplesLen <= 0 {
		return 0
	}
	return int(2.5*SampleRate/float64(samplesLen) + 0.5)
}

// foldChannel flattens one channel's tick data to mono float32 amplitude.
func foldChannel(cd mixing.ChannelData, samplesLen int) []float32 {
	mono := make([]float32, samplesLen)
	pos := 0
	for _, d := range cd {
		vol := float32(d.Volume)
		for _, m := range d.Data {
			if pos >= samplesLen {
				break
			}
			var sum float32
			n := min(m.Channels, len(m.StaticMatrix))
			for c := range n {
				sum += float32(m.StaticMatrix[c])
			}
			if n > 0 {
				mono[pos] = sum / float32(n) * vol
			}
			pos++
		}
	}
	return mono
}

func peak(samples []float32) float32 {
	var pk float32
	for _, s := range samples {
		if s < 0 {
			s = -s
		}
		if s > pk {
			pk = s
		}
	}
	return pk
}

// Stream is the PCM source for ebiten's audio.NewPlayer.
func (p *Player) Stream() *pcmRing { return p.ring }

// Snapshot returns the audible position and levels for this UI frame.
func (p *Player) Snapshot() Snapshot {
	playhead := p.ring.PlayheadSample() - p.LatencyOffset
	if playhead < 0 {
		playhead = 0
	}
	ev := p.states.AtSample(playhead)
	speed := ev.speed
	if speed == 0 {
		speed = p.Info.InitialSpeed
	}
	return Snapshot{
		Playing:   !p.paused.Load() && !p.finished.Load(),
		Finished:  p.finished.Load(),
		Order:     ev.order,
		Row:       ev.row,
		Tick:      ev.tick,
		BPM:       ev.bpm,
		Speed:     speed,
		ChannelVU: ev.vu,
		Playhead:  playhead,
		Underruns: p.ring.Underruns(),
		Buffered:  p.ring.Buffered(),
	}
}

// Scope copies channel ch's amplitude window ending at the audible playhead.
func (p *Player) Scope(ch int, out []float32) {
	if ch < 0 || ch >= len(p.taps) {
		return
	}
	playhead := p.ring.PlayheadSample() - p.LatencyOffset
	p.taps[ch].CopyWindow(playhead, out)
}

// TogglePause flips playback; the render goroutine parks while paused.
func (p *Player) TogglePause() {
	if p.paused.CompareAndSwap(false, true) {
		return
	}
	p.paused.Store(false)
	select {
	case p.resume <- struct{}{}:
	default:
	}
}

// CanSeek reports whether order seeking is supported by this machine.
func (p *Player) CanSeek() bool { return p.seek != nil }

// SeekOrder jumps delta positions in the order list and flushes stale audio.
func (p *Player) SeekOrder(delta int) {
	if p.seek == nil {
		return
	}
	target := int(p.curOrder.Load()) + delta
	if target < 0 {
		target = 0
	}
	if target >= p.Info.NumOrders {
		target = p.Info.NumOrders - 1
	}
	p.mu.Lock()
	err := p.seek.SetOrder(index.Order(target))
	rendered := p.rendered
	p.mu.Unlock()
	if err != nil {
		return
	}
	p.states.Flush()
	p.ring.Flush(rendered)
	for _, t := range p.taps {
		t.Reset(rendered)
	}
}

// Gain returns the user volume (1.0 = unity).
func (p *Player) Gain() float64 { return float64(p.gain.Load()) / 1000 }

// SetGain sets user volume, clamped to [0, 2].
func (p *Player) SetGain(g float64) {
	if g < 0 {
		g = 0
	}
	if g > 2 {
		g = 2
	}
	p.gain.Store(int64(g * 1000))
}

// Close stops the render goroutine and releases the stream.
func (p *Player) Close() {
	p.stopOnce.Do(func() {
		close(p.stop)
		p.paused.Store(false)
		select {
		case p.resume <- struct{}{}:
		default:
		}
		p.ring.Close()
	})
}
