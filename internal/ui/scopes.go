package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/player"
)

const scopeSamples = 1024

// drawScopes renders one oscilloscope box per channel (wrapping to two rows
// beyond 8 channels), fed by the player's audible-position-aligned taps.
func drawScopes(dst *ebiten.Image, p *player.Player, nCh int) {
	cols := nCh
	rows := 1
	if nCh > 8 {
		cols = (nCh + 1) / 2
		rows = 2
	}
	if cols == 0 {
		return
	}
	boxW := float32(W-16) / float32(cols)
	boxH := float32(scopesH) / float32(rows)

	buf := make([]float32, scopeSamples)
	for ch := range nCh {
		col := ch % cols
		row := ch / cols
		x := 8 + float32(col)*boxW
		y := float32(scopesY) + float32(row)*boxH

		vector.FillRect(dst, x+2, y+2, boxW-4, boxH-4, colPanel, false)
		vector.StrokeRect(dst, x+2, y+2, boxW-4, boxH-4, 1, colPanelEdge, false)
		drawText(dst, fmt.Sprintf("%d", ch+1), float64(x)+6, float64(y)+4, colDimmer, 1)

		p.Scope(ch, buf)
		mid := y + boxH/2
		gain := (boxH/2 - 6)
		step := scopeSamples / int(boxW-8)
		if step < 1 {
			step = 1
		}
		var prevX, prevY float32
		first := true
		for i := 0; i < scopeSamples; i += step {
			sx := x + 4 + (boxW-8)*float32(i)/float32(scopeSamples)
			v := buf[i]
			if v > 1 {
				v = 1
			}
			if v < -1 {
				v = -1
			}
			sy := mid - v*gain
			if !first {
				vector.StrokeLine(dst, prevX, prevY, sx, sy, 1, colGreen, false)
			}
			prevX, prevY = sx, sy
			first = false
		}
	}
}
