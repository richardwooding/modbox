package ui

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/player"
)

const scopeSamples = 1024

type rectF struct{ x, y, w, h float32 }

func (r rectF) contains(p image.Point) bool {
	return float32(p.X) >= r.x && float32(p.X) < r.x+r.w &&
		float32(p.Y) >= r.y && float32(p.Y) < r.y+r.h
}

// scopeBoxes lays one box per channel into area, wrapping to two rows beyond
// eight channels. Shared by drawing, tap hit-testing, and demo mode.
func scopeBoxes(nCh int, area rectF) []rectF {
	cols := nCh
	rows := 1
	if nCh > 8 {
		cols = (nCh + 1) / 2
		rows = 2
	}
	if cols == 0 {
		return nil
	}
	boxW := area.w / float32(cols)
	boxH := area.h / float32(rows)
	boxes := make([]rectF, nCh)
	for ch := range nCh {
		boxes[ch] = rectF{
			x: area.x + float32(ch%cols)*boxW,
			y: area.y + float32(ch/cols)*boxH,
			w: boxW,
			h: boxH,
		}
	}
	return boxes
}

// drawScopes renders per-channel oscilloscopes into area, fed by the
// player's audible-position-aligned taps. Muted channels render dimmed.
func drawScopes(dst *ebiten.Image, p *player.Player, nCh int, area rectF) {
	buf := make([]float32, scopeSamples)
	for ch, box := range scopeBoxes(nCh, area) {
		muted := p.IsMuted(ch)

		vector.FillRect(dst, box.x+2, box.y+2, box.w-4, box.h-4, colPanel, false)
		edge := colPanelEdge
		if muted {
			edge = colDimmer
		}
		vector.StrokeRect(dst, box.x+2, box.y+2, box.w-4, box.h-4, 1, edge, false)
		drawText(dst, fmt.Sprintf("%d", ch+1), float64(box.x)+6, float64(box.y)+4, colDimmer, 1)
		if muted {
			drawText(dst, "MUTE", float64(box.x)+float64(box.w)-6-textWidth("MUTE", 1), float64(box.y)+4, colAmber, 1)
		}

		p.Scope(ch, buf)
		line := colGreen
		if muted {
			line = colDimmer
		}
		mid := box.y + box.h/2
		gain := box.h/2 - 6
		step := scopeSamples / int(box.w-8)
		if step < 1 {
			step = 1
		}
		var prevX, prevY float32
		first := true
		for i := 0; i < scopeSamples; i += step {
			sx := box.x + 4 + (box.w-8)*float32(i)/float32(scopeSamples)
			v := buf[i]
			if v > 1 {
				v = 1
			}
			if v < -1 {
				v = -1
			}
			sy := mid - v*gain
			if !first {
				vector.StrokeLine(dst, prevX, prevY, sx, sy, 1, line, false)
			}
			prevX, prevY = sx, sy
			first = false
		}
	}
}
