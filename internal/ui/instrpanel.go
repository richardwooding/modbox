package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/richardwooding/modbox/internal/player"
)

const instFlashFrames = 12

// instrPanel shows the module's instrument names (read from the file bytes;
// scene artists traditionally use them as liner notes) and flashes an entry
// when a channel triggers it.
type instrPanel struct {
	lastTrig map[int]int64 // instrument number (1-based) -> frame of last trigger
	lastRow  int
	lastOrd  int
	scroll   int
}

func newInstrPanel() *instrPanel {
	return &instrPanel{lastTrig: map[int]int64{}, lastRow: -1, lastOrd: -1}
}

// wanted reports whether the panel has anything worth showing.
func (ip *instrPanel) wanted(info player.SongInfo) bool {
	for _, n := range info.Instruments {
		if strings.TrimSpace(n) != "" {
			return true
		}
	}
	return false
}

// Update flags instruments triggered on newly-reached rows by parsing the
// instrument token out of the pre-rendered pattern cells.
func (ip *instrPanel) Update(info player.SongInfo, snap player.Snapshot, frame int64) {
	if snap.Row == ip.lastRow && snap.Order == ip.lastOrd {
		return
	}
	ip.lastRow, ip.lastOrd = snap.Row, snap.Order
	if snap.Order >= len(info.Orders) {
		return
	}
	rows, ok := info.PatternText[info.Orders[snap.Order]]
	if !ok || snap.Row >= len(rows) {
		return
	}
	for _, cell := range rows[snap.Row] {
		fields := strings.Fields(cell)
		if len(fields) < 2 {
			continue
		}
		if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 && n <= len(info.Instruments) {
			ip.lastTrig[n] = frame
			// keep the flashing entry in view
			maxRows := (gridH - 28) / rowH
			if n-1 < ip.scroll {
				ip.scroll = n - 1
			}
			if n-1 >= ip.scroll+maxRows {
				ip.scroll = n - maxRows
			}
		}
	}
}

// Draw renders the panel in the band x..x+w, gridY..gridY+gridH.
func (ip *instrPanel) Draw(dst *ebiten.Image, info player.SongInfo, frame int64, x, w int) {
	vector.FillRect(dst, float32(x), gridY, float32(w), gridH, colPanel, false)
	vector.StrokeRect(dst, float32(x)+0.5, gridY+0.5, float32(w)-1, gridH-1, 1, colPanelEdge, false)
	drawText(dst, "INSTRUMENTS", float64(x)+10, gridY+4, colDimmer, 1)

	maxRows := (gridH - 28) / rowH
	maxChars := (w - 20) / glyphW
	for i := 0; i < maxRows; i++ {
		idx := ip.scroll + i
		if idx >= len(info.Instruments) {
			break
		}
		y := float64(gridY + 22 + i*rowH)
		line := fmt.Sprintf("%02d %s", idx+1, info.Instruments[idx])
		if len(line) > maxChars && maxChars > 1 {
			line = line[:maxChars-1] + "…"
		}
		col := colDim
		if age := frame - ip.lastTrig[idx+1]; ip.lastTrig[idx+1] > 0 && age < instFlashFrames {
			// flash: accent bar decaying with age
			alpha := 1 - float32(age)/instFlashFrames
			vector.FillRect(dst, float32(x)+4, float32(y)-2, float32(w)-8, rowH, fade(colAccentDim, 1-alpha), false)
			col = colText
		}
		drawText(dst, line, float64(x)+10, y, col, 1)
	}
	if len(info.Instruments) > maxRows {
		more := fmt.Sprintf("%d/%d", min(ip.scroll+maxRows, len(info.Instruments)), len(info.Instruments))
		drawText(dst, more, float64(x+w)-10-textWidth(more, 1), gridY+4, colDimmer, 1)
	}
}
