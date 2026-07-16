package ui

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
}

func newPlayerScene(g *Game, p *player.Player) (*playerScene, error) {
	ap, err := g.audioCtx.NewPlayer(p.Stream())
	if err != nil {
		return nil, fmt.Errorf("audio: %w", err)
	}
	ap.SetBufferSize(100 * time.Millisecond)
	p.Start()
	ap.Play()
	return &playerScene{p: p, audio: ap, vu: newVUMeters(p.Info.NumChannels)}, nil
}

func (s *playerScene) Update(g *Game) error {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		s.p.TogglePause()
		if s.audio.IsPlaying() {
			s.audio.Pause()
		} else {
			s.audio.Play()
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		s.p.SeekOrder(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		s.p.SeekOrder(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyEqual), inpututil.IsKeyJustPressed(ebiten.KeyKPAdd):
		s.p.SetGain(s.p.Gain() + 0.1)
	case inpututil.IsKeyJustPressed(ebiten.KeyMinus), inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract):
		s.p.SetGain(s.p.Gain() - 0.1)
	case inpututil.IsKeyJustPressed(ebiten.KeyD):
		s.debug = !s.debug
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		s.close()
		g.scene = newDropScene()
		return nil
	}

	// Dropping a new file replaces the current player.
	if files := ebiten.DroppedFiles(); files != nil {
		if data, _, ok := firstFile(files); ok {
			if np, err := player.Load(data); err == nil {
				if ns, err := newPlayerScene(g, np); err == nil {
					s.close()
					g.scene = ns
					return nil
				}
				np.Close()
			}
		}
	}

	s.vu.Update(s.p.Snapshot().ChannelVU)
	return nil
}

func (s *playerScene) close() {
	_ = s.audio.Close()
	s.p.Close()
}

func (s *playerScene) Draw(dst *ebiten.Image) {
	snap := s.p.Snapshot()
	info := s.p.Info

	// Header
	vector.FillRect(dst, 0, 0, W, headerH, colPanel, false)
	vector.StrokeLine(dst, 0, headerH, W, headerH, 1, colPanelEdge, false)
	name := info.Name
	if name == "" {
		name = "(untitled)"
	}
	drawText(dst, name, 16, 10, colText, 2)
	badge := fmt.Sprintf("[%s]", info.Format)
	drawText(dst, badge, 16+textWidth(name, 2)+12, 16, colAccent, 1)

	stats := fmt.Sprintf("ORD %02d/%02d  ROW %02d  BPM %3d  SPD %d  VOL %.0f%%",
		snap.Order, info.NumOrders, snap.Row, snap.BPM, snap.Speed, s.p.Gain()*100)
	drawText(dst, stats, 16, 38, colDim, 1)
	state := "▶ PLAYING"
	stateCol := colGreen
	if snap.Finished {
		state, stateCol = "■ FINISHED", colDim
	} else if !snap.Playing {
		state, stateCol = "⏸ PAUSED", colAmber
	}
	drawText(dst, state, W-16-textWidth(state, 1), 38, stateCol, 1)

	// Order strip
	drawOrderStrip(dst, info, snap.Order)

	// Pattern grid
	drawPatternGrid(dst, info, snap)

	// Scopes
	drawScopes(dst, s.p, info.NumChannels)

	// VU
	s.vu.Draw(dst)

	// Footer / debug
	if s.debug {
		dbg := fmt.Sprintf("underruns %d  buffered %dB  playhead %d  latency %d",
			snap.Underruns, snap.Buffered, snap.Playhead, s.p.LatencyOffset)
		drawText(dst, dbg, 16, H-18, colAmber, 1)
	} else {
		help := "space pause · ←/→ seek order · +/- volume · esc back · d debug"
		drawText(dst, help, (W-textWidth(help, 1))/2, H-18, colDimmer, 1)
	}
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
