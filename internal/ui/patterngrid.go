package ui

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/player"
)

const (
	maxGridChannels = 8
	rowH            = 16
)

// drawPatternGrid renders the scrolling pattern view into the leftmost width
// pixels: the current row sits on a fixed accent bar while the text glides
// underneath it, offset by snap.RowFrac (sub-row smooth scrolling).
func drawPatternGrid(dst *ebiten.Image, info player.SongInfo, snap player.Snapshot, width int) {
	vector.FillRect(dst, 0, gridY, float32(width), gridH, colPanel, false)
	vector.StrokeRect(dst, 0.5, gridY+0.5, float32(width)-1, gridH-1, 1, colPanelEdge, false)

	if snap.Order >= len(info.Orders) {
		return
	}
	rows, ok := info.PatternText[info.Orders[snap.Order]]
	if !ok || len(rows) == 0 {
		return
	}

	nCh := min(info.NumChannels, maxGridChannels)
	cellW := (width - 56) / nCh
	centerY := gridY + gridH/2 - rowH/2
	visible := gridH / rowH / 2 // rows above and below center

	// Highlight bar for the current row — fixed; the text moves.
	vector.FillRect(dst, 0, float32(centerY)-2, float32(width), rowH+2, colAccentDim, false)

	// Clip row text to the panel so glided rows don't bleed into the
	// header or scopes.
	clip := dst.SubImage(image.Rect(0, gridY+2, width, gridY+gridH-2)).(*ebiten.Image)
	scroll := snap.RowFrac * rowH

	for off := -visible - 1; off <= visible+1; off++ {
		r := snap.Row + off
		if r < 0 || r >= len(rows) {
			continue
		}
		y := float64(centerY+off*rowH) - scroll
		dist := float32(abs(off)) / float32(visible+1)

		numCol := fade(colDim, dist)
		if r%4 == 0 {
			numCol = fade(colAccent, dist*1.2)
		}
		if off == 0 {
			numCol = colAccent
		}
		drawText(clip, fmt.Sprintf("%02d", r), 12, y, numCol, 1)

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
			drawText(clip, cell, x, y, txtCol, 1)
		}
	}

	// Channel separators + channel numbers pinned to the panel top.
	for ch := 0; ch <= nCh; ch++ {
		x := float32(48 + ch*cellW)
		if x > float32(width) {
			break
		}
		vector.StrokeLine(dst, x-4, gridY, x-4, gridY+gridH, 1, colPanelEdge, false)
		if ch < nCh {
			drawText(dst, fmt.Sprintf("CH%d", ch+1), float64(x), gridY+4, colDimmer, 1)
		}
	}
	if info.NumChannels > maxGridChannels {
		note := fmt.Sprintf("showing %d of %d channels", maxGridChannels, info.NumChannels)
		drawText(dst, note, float64(width)-16-textWidth(note, 1), gridY+gridH-18, colDimmer, 1)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
