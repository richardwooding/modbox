package ui

import (
	"fmt"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/modules"
	"github.com/richardwooding/modbox/internal/player"
)

// dropScene is the landing screen: wordmark, drop hint, and the bundled
// demo-song list. The first click/keypress doubles as the browser audio
// gesture — the audio.Player is only created from inside Update.
type dropScene struct {
	selected  int
	errMsg    string
	autoTried bool
}

func newDropScene() *dropScene { return &dropScene{} }

func (s *dropScene) Update(g *Game) error {
	demos := modules.Demos()

	// ?demo=N deep-links straight into a bundled song; &fx=1 adds the
	// spectacle view.
	if !s.autoTried {
		s.autoTried = true
		if n := autostartDemo(); n >= 0 && n < len(demos) {
			s.selected = n
			s.loadDemo(g, n)
			if ps, ok := g.scene.(*playerScene); ok && autostartFX() {
				ps.demoMode = true
			}
			return nil
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && s.selected < len(demos)-1 {
		s.selected++
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) && s.selected > 0 {
		s.selected--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if len(demos) > 0 {
			s.loadDemo(g, s.selected)
		}
	}
	// Taps (mouse or touch): demo rows select on first tap, play on second;
	// the browse button opens the file dialog.
	if taps := justTaps(); len(taps) > 0 {
		if canPickFiles() && s.browseBtn().hit(taps) {
			openFilePicker()
		}
		for _, pt := range taps {
			for i := range demos {
				y := demoListY + i*demoRowH
				if pt.Y >= y && pt.Y < y+demoRowH && pt.X >= 160 && pt.X < W-160 {
					if s.selected == i {
						s.loadDemo(g, i)
					}
					s.selected = i
				}
			}
		}
	}

	// Module file arriving via drag-and-drop or the browse dialog.
	if data, name, ok := takePickedFile(); ok {
		s.load(g, data, name)
	} else if files := ebiten.DroppedFiles(); files != nil {
		if data, name, ok := firstFile(files); ok {
			s.load(g, data, name)
		}
	}
	return nil
}

// browseBtn is the phone-friendly alternative to drag-and-drop.
func (s *dropScene) browseBtn() button {
	return button{x: W/2 - 90, y: demoListY + float32(len(modules.Demos()))*demoRowH + 18, w: 180, h: 40, label: "browse files…"}
}

func (s *dropScene) load(g *Game, data []byte, name string) {
	p, err := player.Load(data)
	if err != nil {
		s.errMsg = fmt.Sprintf("%s: %v", name, err)
		return
	}
	ps, err := newPlayerScene(g, p)
	if err != nil {
		p.Close()
		s.errMsg = err.Error()
		return
	}
	g.scene = ps
}

// loadDemo starts bundled demo i and remembers the index so the jukebox can
// advance to the next demo when the song ends.
func (s *dropScene) loadDemo(g *Game, i int) {
	demos := modules.Demos()
	s.load(g, demos[i].Data, demos[i].Title)
	if ps, ok := g.scene.(*playerScene); ok {
		ps.demoIdx = i
	}
}

// firstFile reads the first regular file from a dropped fs.FS.
func firstFile(files fs.FS) (data []byte, name string, ok bool) {
	_ = fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || ok {
			return nil
		}
		if b, rerr := fs.ReadFile(files, path); rerr == nil {
			data, name, ok = b, d.Name(), true
		}
		return nil
	})
	return data, name, ok
}

const (
	demoListY = 330
	demoRowH  = 40
)

func (s *dropScene) Draw(dst *ebiten.Image) {
	// Wordmark: MODBOX with bar-chart-ish accent blocks.
	title := "MODBOX"
	scale := 6.0
	tw := textWidth(title, scale)
	drawText(dst, title, (W-tw)/2, 90, colAccent, scale)
	sub := "a tracker-music player, 100% Go, in your browser"
	drawText(dst, sub, (W-textWidth(sub, 2))/2, 180, colDim, 2)

	hint := "drag a .mod / .s3m / .xm / .it anywhere — or pick a demo:"
	drawText(dst, hint, (W-textWidth(hint, 2))/2, 260, colText, 2)

	demos := modules.Demos()
	for i, d := range demos {
		y := float64(demoListY + i*demoRowH)
		line := fmt.Sprintf("%s — %s", d.Title, d.Artist)
		lic := d.License
		if i == s.selected {
			vector.FillRect(dst, 160, float32(y)-6, W-320, demoRowH-8, colPanel, false)
			vector.StrokeRect(dst, 160, float32(y)-6, W-320, demoRowH-8, 1, colAccent, false)
			drawText(dst, "▶", 180, y, colAccent, 2)
		}
		drawText(dst, line, 220, y, colText, 2)
		drawText(dst, lic, W-180-textWidth(lic, 1), y+6, colDim, 1)
	}

	if canPickFiles() {
		s.browseBtn().draw(dst, true)
	}

	if s.errMsg != "" {
		drawText(dst, s.errMsg, (W-textWidth(s.errMsg, 1))/2, H-70, colRed, 1)
	}
	foot := "tap/enter to play · music credits in NOTICE.md"
	drawText(dst, foot, (W-textWidth(foot, 1))/2, H-36, colDimmer, 1)
}
