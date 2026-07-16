package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/player"
)

const (
	maxGridChannels = 8
	rowH            = 16
)

// drawPatternGrid renders the scrolling pattern view with the current row
// centered on an accent bar and other rows dimmed by distance.
func drawPatternGrid(dst *ebiten.Image, info player.SongInfo, snap player.Snapshot) {
	vector.FillRect(dst, 0, gridY, W, gridH, colPanel, false)
	vector.StrokeRect(dst, 0.5, gridY+0.5, W-1, gridH-1, 1, colPanelEdge, false)

	if snap.Order >= len(info.Orders) {
		return
	}
	rows, ok := info.PatternText[info.Orders[snap.Order]]
	if !ok || len(rows) == 0 {
		return
	}

	nCh := min(info.NumChannels, maxGridChannels)
	cellW := (W - 56) / nCh
	centerY := gridY + gridH/2 - rowH/2
	visible := gridH / rowH / 2 // rows above and below center

	// Highlight bar for the current row.
	vector.FillRect(dst, 0, float32(centerY)-2, W, rowH+2, colAccentDim, false)

	for off := -visible; off <= visible; off++ {
		r := snap.Row + off
		if r < 0 || r >= len(rows) {
			continue
		}
		y := float64(centerY + off*rowH)
		dist := float32(abs(off)) / float32(visible+1)

		numCol := fade(colDim, dist)
		if r%4 == 0 {
			numCol = fade(colAccent, dist*1.2)
		}
		if off == 0 {
			numCol = colAccent
		}
		drawText(dst, fmt.Sprintf("%02d", r), 12, y, numCol, 1)

		cells := rows[r]
		for ch := 0; ch < nCh && ch < len(cells); ch++ {
			x := float64(48 + ch*cellW)
			txtCol := fade(colText, dist)
			if off == 0 {
				txtCol = colText
			}
			cell := cells[ch]
			maxChars := cellW/glyphW - 1
			if len(cell) > maxChars && maxChars > 0 {
				cell = cell[:maxChars]
			}
			drawText(dst, cell, x, y, txtCol, 1)
		}
	}

	// Channel separators + channel numbers pinned to the panel top.
	for ch := 0; ch <= nCh; ch++ {
		x := float32(48 + ch*cellW)
		vector.StrokeLine(dst, x-4, gridY, x-4, gridY+gridH, 1, colPanelEdge, false)
		if ch < nCh {
			drawText(dst, fmt.Sprintf("CH%d", ch+1), float64(x), gridY+4, colDimmer, 1)
		}
	}
	if info.NumChannels > maxGridChannels {
		note := fmt.Sprintf("showing %d of %d channels", maxGridChannels, info.NumChannels)
		drawText(dst, note, W-16-textWidth(note, 1), gridY+gridH-18, colDimmer, 1)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
