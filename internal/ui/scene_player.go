package ui

import (
	"fmt"
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/modules"
	"github.com/richardwooding/modbox/internal/player"
)

// Layout bands (logical pixels).
const (
	headerH = 64
	orderH  = 28
	gridY   = headerH + orderH
	gridH   = 320
	scopesY = gridY + gridH + 8
	scopesH = 120
	vuY     = scopesY + scopesH + 8
	vuH     = 26
)

type playerScene struct {
	p     *player.Player
	audio *audio.Player
	vu    *vuMeters
	debug bool

	demoIdx  int // bundled-demo index this song came from; -1 for user files
	demoMode bool
	spec     *spectrum
	specBuf  []float32
	master   []float32
	instr    *instrPanel // nil when the module has no usable names

	frame        int64 // Update counter, for double-tap detection
	lastTapCh    int
	lastTapFrame int64

	// transport bar (top-right of the header); btnLoad is wasm-only
	btnLoad, btnPrev, btnPlay, btnNext, btnVolDn, btnVolUp, btnLoop, btnFull button
}

func newPlayerScene(g *Game, p *player.Player) (*playerScene, error) {
	ap, err := g.audioCtx.NewPlayer(p.Stream())
	if err != nil {
		return nil, fmt.Errorf("audio: %w", err)
	}
	ap.SetBufferSize(100 * time.Millisecond)
	p.Start()
	ap.Play()
	s := &playerScene{
		p:         p,
		audio:     ap,
		vu:        newVUMeters(p.Info.NumChannels),
		demoIdx:   -1,
		spec:      newSpectrum(),
		specBuf:   make([]float32, fftSize),
		master:    make([]float32, fftSize),
		lastTapCh: -1,
	}
	if ip := newInstrPanel(); ip.wanted(p.Info) {
		s.instr = ip
	}
	s.layoutTransport()
	return s, nil
}

// gridWidth leaves room for the instrument panel when there is one.
func (s *playerScene) gridWidth() int {
	if s.instr != nil {
		return W - instrPanelW
	}
	return W
}

const instrPanelW = 240

// layoutTransport places the tap targets right-to-left in the header.
func (s *playerScene) layoutTransport() {
	const bw, bh, gap = 44, 30, 6
	x := float32(W - 16 - bw)
	place := func(b *button, label string) {
		*b = button{x: x, y: 6, w: bw, h: bh, label: label}
		x -= bw + gap
	}
	// Labels stick to glyphs the 6x13 bitmap font actually has.
	place(&s.btnFull, "full")
	place(&s.btnLoop, "loop")
	place(&s.btnVolUp, "+")
	place(&s.btnVolDn, "-")
	place(&s.btnNext, ">>")
	place(&s.btnPlay, "||")
	place(&s.btnPrev, "<<")
	if canPickFiles() {
		place(&s.btnLoad, "load")
	}
}

// playerScopeArea is the scope band in the normal layout.
func playerScopeArea() rectF { return rectF{x: 8, y: scopesY, w: W - 16, h: scopesH} }

// setDemoMode toggles the fullscreen spectacle view. The trigger is always a
// user gesture (keypress or tap), which browsers require for fullscreen.
func (s *playerScene) setDemoMode(on bool) {
	s.demoMode = on
	ebiten.SetFullscreen(on)
}

// togglePlayback pauses/resumes both the renderer and the audio device.
func (s *playerScene) togglePlayback() {
	s.p.TogglePause()
	if s.audio.IsPlaying() {
		s.audio.Pause()
	} else {
		s.audio.Play()
	}
}

func (s *playerScene) Update(g *Game) error {
	s.frame++

	if s.demoMode {
		s.updateDemoMode(g)
		return nil
	}
	if s.handleKeys(g) {
		return nil // left for the song picker
	}
	s.handleDigitMutes()
	s.handleTaps(g)
	s.jukebox(g)
	if g.scene != scene(s) {
		return nil // jukebox advanced; this scene (and its player) is closed
	}
	if s.handleIncomingFile(g) {
		return nil // replaced by a fresh player scene
	}

	snap := s.p.Snapshot()
	s.vu.Update(snap.ChannelVU)
	if s.instr != nil {
		s.instr.Update(s.p.Info, snap, s.frame)
	}
	return nil
}

// updateDemoMode is the spectacle view: any tap, F, or Esc returns to the
// player; space still pauses.
func (s *playerScene) updateDemoMode(g *Game) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeyF) ||
		len(justTaps()) > 0 {
		s.setDemoMode(false)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		s.togglePlayback()
	}
	s.updateSpectrum()
	s.jukebox(g)
}

// handleKeys runs the keyboard transport; reports whether the scene exited.
func (s *playerScene) handleKeys(g *Game) bool {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		s.togglePlayback()
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		s.p.SeekOrder(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		s.p.SeekOrder(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyEqual), inpututil.IsKeyJustPressed(ebiten.KeyKPAdd):
		s.p.SetGain(s.p.Gain() + 0.1)
	case inpututil.IsKeyJustPressed(ebiten.KeyMinus), inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract):
		s.p.SetGain(s.p.Gain() - 0.1)
	case inpututil.IsKeyJustPressed(ebiten.KeyL):
		s.p.SetLoop(!s.p.Loop())
	case inpututil.IsKeyJustPressed(ebiten.KeyF):
		s.setDemoMode(true)
	case inpututil.IsKeyJustPressed(ebiten.KeyD):
		s.debug = !s.debug
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		s.close()
		g.scene = newDropScene()
		return true
	}
	return false
}

// handleDigitMutes maps digit keys 1-9 (and 0 as channel 10) to mutes.
func (s *playerScene) handleDigitMutes() {
	if !s.p.CanMute() {
		return
	}
	for i, k := range []ebiten.Key{
		ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4,
		ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8,
		ebiten.KeyDigit9, ebiten.KeyDigit0,
	} {
		if inpututil.IsKeyJustPressed(k) {
			s.p.ToggleMute(i)
		}
	}
}

// handleTaps routes transport-bar, scope, and pattern-grid taps.
func (s *playerScene) handleTaps(g *Game) {
	taps := justTaps()
	if len(taps) == 0 {
		return
	}
	switch {
	case canPickFiles() && s.btnLoad.hit(taps):
		openFilePicker()
	case s.btnPrev.hit(taps):
		s.p.SeekOrder(-1)
	case s.btnPlay.hit(taps):
		s.togglePlayback()
	case s.btnNext.hit(taps):
		s.p.SeekOrder(1)
	case s.btnVolDn.hit(taps):
		s.p.SetGain(s.p.Gain() - 0.1)
	case s.btnVolUp.hit(taps):
		s.p.SetGain(s.p.Gain() + 0.1)
	case s.btnLoop.hit(taps):
		s.p.SetLoop(!s.p.Loop())
	case s.btnFull.hit(taps):
		s.setDemoMode(true)
	case s.scopeTap(taps):
		// handled: mute (single tap) or solo (double tap)
	case s.gridTapped(taps):
		// a tap on the pattern grid is a big, phone-friendly pause target
		s.togglePlayback()
	}
}

// gridTapped reports whether any tap landed on the pattern grid.
func (s *playerScene) gridTapped(taps []image.Point) bool {
	for _, pt := range taps {
		if pt.Y >= gridY && pt.Y < gridY+gridH {
			return true
		}
	}
	return false
}

// handleIncomingFile replaces the player with a dropped or browsed module;
// reports whether the scene was replaced.
func (s *playerScene) handleIncomingFile(g *Game) bool {
	data, _, ok := takePickedFile()
	if !ok {
		if files := ebiten.DroppedFiles(); files != nil {
			data, _, ok = firstFile(files)
		}
	}
	if !ok {
		return false
	}
	np, err := player.Load(data)
	if err != nil {
		return false
	}
	ns, err := newPlayerScene(g, np)
	if err != nil {
		np.Close()
		return false
	}
	s.close()
	g.scene = ns
	return true
}

// scopeTap mutes a channel whose scope box was tapped; a double tap (within
// ~20 frames on the same box) solos it instead. Reports whether a tap landed.
func (s *playerScene) scopeTap(taps []image.Point) bool {
	if !s.p.CanMute() {
		return false
	}
	boxes := scopeBoxes(s.p.Info.NumChannels, playerScopeArea())
	for _, pt := range taps {
		for ch, box := range boxes {
			if !box.contains(pt) {
				continue
			}
			if ch == s.lastTapCh && s.frame-s.lastTapFrame <= 20 {
				s.p.Solo(ch)
			} else {
				s.p.ToggleMute(ch)
			}
			s.lastTapCh, s.lastTapFrame = ch, s.frame
			return true
		}
	}
	return false
}

// jukebox auto-advances to the next bundled demo when a demo song finishes
// with looping off.
func (s *playerScene) jukebox(g *Game) {
	if s.demoIdx < 0 || s.p.Loop() || !s.p.Snapshot().Finished {
		return
	}
	demos := modules.Demos()
	if len(demos) == 0 {
		return
	}
	next := (s.demoIdx + 1) % len(demos)
	np, err := player.Load(demos[next].Data)
	if err != nil {
		return
	}
	ns, err := newPlayerScene(g, np)
	if err != nil {
		np.Close()
		return
	}
	ns.demoIdx = next
	ns.demoMode = s.demoMode // stay in the spectacle if we were there
	s.close()
	g.scene = ns
}

// updateSpectrum folds all channel taps into a master window and feeds the
// analyzer.
func (s *playerScene) updateSpectrum() {
	for i := range s.master {
		s.master[i] = 0
	}
	for ch := range s.p.Info.NumChannels {
		s.p.Scope(ch, s.specBuf)
		for i, v := range s.specBuf {
			s.master[i] += v
		}
	}
	s.spec.Update(s.master)
}

func (s *playerScene) close() {
	_ = s.audio.Close()
	s.p.Close()
}

func (s *playerScene) Draw(dst *ebiten.Image) {
	if s.demoMode {
		s.drawDemoMode(dst)
		return
	}
	snap := s.p.Snapshot()
	info := s.p.Info

	// Header
	vector.FillRect(dst, 0, 0, W, headerH, colPanel, false)
	vector.StrokeLine(dst, 0, headerH, W, headerH, 1, colPanelEdge, false)
	name := info.Name
	if name == "" {
		name = "(untitled)"
	}
	// Keep the title clear of the transport bar.
	maxName := int((float64(s.btnPrev.x) - 80) / (glyphW * 2))
	if canPickFiles() {
		maxName = int((float64(s.btnLoad.x) - 80) / (glyphW * 2))
	}
	if len(name) > maxName && maxName > 1 {
		name = name[:maxName-1] + "…"
	}
	drawText(dst, name, 16, 10, colText, 2)
	badge := fmt.Sprintf("[%s]", info.Format)
	drawText(dst, badge, 16+textWidth(name, 2)+12, 16, colAccent, 1)

	stats := fmt.Sprintf("ORD %02d/%02d  ROW %02d  BPM %3d  SPD %d  VOL %.0f%%",
		snap.Order, info.NumOrders, snap.Row, snap.BPM, snap.Speed, s.p.Gain()*100)
	if s.p.Loop() {
		stats += "  LOOP"
	}
	drawText(dst, stats, 16, 38, colDim, 1)
	state := "▶ PLAYING"
	stateCol := colGreen
	if snap.Finished {
		state, stateCol = "■ FINISHED", colDim
	} else if !snap.Playing {
		state, stateCol = "⏸ PAUSED", colAmber
	}
	drawText(dst, state, W-16-textWidth(state, 1), 42, stateCol, 1)

	// Transport bar; the play button mirrors state (|| = will pause).
	if canPickFiles() {
		s.btnLoad.draw(dst, false)
	}
	s.btnPrev.draw(dst, false)
	if snap.Playing {
		s.btnPlay.label = "||"
	} else {
		s.btnPlay.label = "▶"
	}
	s.btnPlay.draw(dst, !snap.Playing)
	s.btnNext.draw(dst, false)
	s.btnVolDn.draw(dst, false)
	s.btnVolUp.draw(dst, false)
	s.btnLoop.draw(dst, s.p.Loop())
	s.btnFull.draw(dst, false)

	// Order strip
	drawOrderStrip(dst, info, snap.Order)

	// Pattern grid + instrument panel
	drawPatternGrid(dst, info, snap, s.gridWidth())
	if s.instr != nil {
		s.instr.Draw(dst, info, s.frame, s.gridWidth(), instrPanelW)
	}

	// Scopes
	drawScopes(dst, s.p, info.NumChannels, playerScopeArea())

	// VU
	s.vu.Draw(dst)

	// Footer / debug
	if s.debug {
		dbg := fmt.Sprintf("underruns %d  buffered %dB  playhead %d  latency %d",
			snap.Underruns, snap.Buffered, snap.Playhead, s.p.LatencyOffset)
		drawText(dst, dbg, 16, H-18, colAmber, 1)
	} else {
		help := "space pause · ←/→ seek · tap scope mute, 2x solo · l loop · f fullscreen · esc back"
		drawText(dst, help, (W-textWidth(help, 1))/2, H-18, colDimmer, 1)
	}
}

// drawDemoMode is the fullscreen spectacle: giant scopes over a spectrum
// analyzer, chrome hidden.
func (s *playerScene) drawDemoMode(dst *ebiten.Image) {
	info := s.p.Info

	name := info.Name
	if name == "" {
		name = "(untitled)"
	}
	drawText(dst, name, (W-textWidth(name, 3))/2, 18, colAccent, 3)

	const specH = 150
	scopeArea := rectF{x: 8, y: 70, w: W - 16, h: H - 70 - specH - 40}
	drawScopes(dst, s.p, info.NumChannels, scopeArea)

	// Spectrum bars across the bottom.
	barsY := float32(H - specH - 24)
	barW := float32(W-16) / spectrumBars
	for b, v := range s.spec.bars {
		h := v * specH
		x := 8 + float32(b)*barW
		vector.FillRect(dst, x+1, barsY+specH-h, barW-2, h, colAccent, false)
		vector.FillRect(dst, x+1, barsY+specH-h, barW-2, min32(h, 3), colText, false)
	}

	hint := "tap / esc to exit"
	drawText(dst, hint, (W-textWidth(hint, 1))/2, H-16, colDimmer, 1)
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func drawOrderStrip(dst *ebiten.Image, info player.SongInfo, cur int) {
	y := float64(headerH) + 7
	x := 16.0
	for i, pat := range info.Orders {
		cell := fmt.Sprintf("%02d", pat)
		if i == cur {
			vector.FillRect(dst, float32(x)-3, float32(y)-3, float32(textWidth(cell, 1))+6, 18, colAccentDim, false)
			drawText(dst, cell, x, y, colText, 1)
		} else {
			drawText(dst, cell, x, y, colDim, 1)
		}
		x += textWidth(cell, 1) + 10
		if x > W-40 {
			drawText(dst, "…", x, y, colDim, 1)
			break
		}
	}
}
