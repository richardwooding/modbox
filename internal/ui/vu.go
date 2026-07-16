package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// vuMeters renders per-channel level bars with fast attack, slow decay, and
// amber peak-hold ticks.
type vuMeters struct {
	levels []float32
	peaks  []float32
}

func newVUMeters(n int) *vuMeters {
	return &vuMeters{levels: make([]float32, n), peaks: make([]float32, n)}
}

const (
	vuDecay   = 0.94  // per frame (~60fps): ~12 dB/s
	peakDecay = 0.995 // peak hold falls slowly
)

func (v *vuMeters) Update(vu []float32) {
	for i := range v.levels {
		var target float32
		if i < len(vu) {
			target = vu[i]
			if target > 1 {
				target = 1
			}
		}
		if target > v.levels[i] {
			v.levels[i] = target // instant attack
		} else {
			v.levels[i] *= vuDecay
		}
		if v.levels[i] > v.peaks[i] {
			v.peaks[i] = v.levels[i]
		} else {
			v.peaks[i] *= peakDecay
		}
	}
}

func (v *vuMeters) Draw(dst *ebiten.Image) {
	n := len(v.levels)
	if n == 0 {
		return
	}
	barW := float32(W-16) / float32(n)
	for i, lvl := range v.levels {
		x := 8 + float32(i)*barW
		w := barW - 6
		vector.FillRect(dst, x, vuY, w, vuH, colPanel, false)

		h := lvl * vuH
		clr := colGreen
		if lvl > 0.85 {
			clr = colAmber
		}
		vector.FillRect(dst, x, vuY+vuH-h, w, h, clr, false)

		ph := v.peaks[i] * vuH
		if ph > 1 {
			vector.FillRect(dst, x, vuY+vuH-ph-1, w, 2, colAmber, false)
		}
	}
}
