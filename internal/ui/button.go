package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// justTaps returns every pointer-down this frame — mouse clicks and touch
// starts unified, in logical coordinates. Ebiten does not synthesize clicks
// from touches, so mobile needs both handled explicitly.
func justTaps() []image.Point {
	var pts []image.Point
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		pts = append(pts, image.Pt(x, y))
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		pts = append(pts, image.Pt(x, y))
	}
	return pts
}

// button is a minimal immediate-mode tap target.
type button struct {
	x, y, w, h float32
	label      string
}

func (b button) hit(pts []image.Point) bool {
	for _, p := range pts {
		if float32(p.X) >= b.x && float32(p.X) < b.x+b.w &&
			float32(p.Y) >= b.y && float32(p.Y) < b.y+b.h {
			return true
		}
	}
	return false
}

func (b button) draw(dst *ebiten.Image, accent bool) {
	vector.FillRect(dst, b.x, b.y, b.w, b.h, colPanel, false)
	edge := colPanelEdge
	txt := colText
	if accent {
		edge = colAccent
		txt = colAccent
	}
	vector.StrokeRect(dst, b.x, b.y, b.w, b.h, 1, edge, false)
	tw := textWidth(b.label, 1)
	drawText(dst, b.label, float64(b.x)+(float64(b.w)-tw)/2, float64(b.y)+(float64(b.h)-glyphH)/2, txt, 1)
}
