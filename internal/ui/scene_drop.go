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

	// ?demo=N deep-links straight into a bundled song.
	if !s.autoTried {
		s.autoTried = true
		if n := autostartDemo(); n >= 0 && n < len(demos) {
			s.selected = n
			s.load(g, demos[n].Data, demos[n].Title)
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
			s.load(g, demos[s.selected].Data, demos[s.selected].Title)
		}
	}
	// Click on a demo row selects/starts it.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		_, my := ebiten.CursorPosition()
		for i := range demos {
			y := demoListY + i*demoRowH
			if my >= y && my < y+demoRowH {
				if s.selected == i {
					s.load(g, demos[i].Data, demos[i].Title)
				}
				s.selected = i
			}
		}
	}

	// Dropped module file.
	if files := ebiten.DroppedFiles(); files != nil {
		if data, name, ok := firstFile(files); ok {
			s.load(g, data, name)
		}
	}
	return nil
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
			vector.DrawFilledRect(dst, 160, float32(y)-6, W-320, demoRowH-8, colPanel, false)
			vector.StrokeRect(dst, 160, float32(y)-6, W-320, demoRowH-8, 1, colAccent, false)
			drawText(dst, "▶", 180, y, colAccent, 2)
		}
		drawText(dst, line, 220, y, colText, 2)
		drawText(dst, lic, W-180-textWidth(lic, 1), y+6, colDim, 1)
	}

	if s.errMsg != "" {
		drawText(dst, s.errMsg, (W-textWidth(s.errMsg, 1))/2, H-70, colRed, 1)
	}
	foot := "enter/click to play · music credits in NOTICE.md"
	drawText(dst, foot, (W-textWidth(foot, 1))/2, H-36, colDimmer, 1)
}
