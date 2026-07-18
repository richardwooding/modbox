package ui

import (
	"fmt"
	"image"
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
	if s.tryAutostart(g, demos) {
		return nil
	}
	s.handleKeys(g, demos)
	if g.scene != scene(s) {
		return nil // a song started; don't keep driving the stale scene
	}
	s.handleTaps(g, demos)
	if g.scene != scene(s) {
		return nil
	}
	s.handleIncomingFile(g)
	return nil
}

// tryAutostart honors ?demo=N (and &fx=1) once; reports whether a song started.
func (s *dropScene) tryAutostart(g *Game, demos []modules.Demo) bool {
	if s.autoTried {
		return false
	}
	s.autoTried = true
	n := autostartDemo()
	if n < 0 || n >= len(demos) {
		return false
	}
	s.selected = n
	s.loadDemo(g, n)
	if ps, ok := g.scene.(*playerScene); ok && autostartFX() {
		ps.demoMode = true
	}
	return true
}

// handleKeys moves the selection and starts on enter/space.
func (s *dropScene) handleKeys(g *Game, demos []modules.Demo) {
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
}

// handleTaps routes touch/click: the browse button opens the file dialog;
// demo rows select on first tap, play on second.
func (s *dropScene) handleTaps(g *Game, demos []modules.Demo) {
	taps := justTaps()
	if len(taps) == 0 {
		return
	}
	if canPickFiles() && s.browseBtn().hit(taps) {
		openFilePicker()
	}
	for _, pt := range taps {
		if g.scene != scene(s) {
			break // a song already started; further taps would leak players
		}
		i, ok := demoRowAt(pt, len(demos))
		if !ok {
			continue
		}
		if s.selected == i {
			s.loadDemo(g, i)
		}
		s.selected = i
	}
}

// demoRowAt maps a point to a demo-list row index.
func demoRowAt(pt image.Point, n int) (int, bool) {
	if pt.X < 160 || pt.X >= W-160 || pt.Y < demoListY {
		return 0, false
	}
	i := (pt.Y - demoListY) / demoRowH
	if i >= n {
		return 0, false
	}
	return i, true
}

// handleIncomingFile loads a module arriving via drag-and-drop or the
// browse dialog.
func (s *dropScene) handleIncomingFile(g *Game) {
	if data, name, ok := takePickedFile(); ok {
		s.load(g, data, name)
		return
	}
	if files := ebiten.DroppedFiles(); files != nil {
		if data, name, ok := firstFile(files); ok {
			s.load(g, data, name)
		}
	}
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
