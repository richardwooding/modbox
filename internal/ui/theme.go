package ui

import "image/color"

// gloam palette, mapped onto a FastTracker 2 layout.
var (
	colBG        = color.RGBA{0x0b, 0x0a, 0x10, 0xff} // near-black
	colPanel     = color.RGBA{0x14, 0x12, 0x21, 0xff}
	colPanelEdge = color.RGBA{0x2a, 0x26, 0x40, 0xff}
	colAccent    = color.RGBA{0xa7, 0x8b, 0xfa, 0xff} // purple
	colAccentDim = color.RGBA{0x4c, 0x1d, 0x95, 0xff}
	colText      = color.RGBA{0xe6, 0xe1, 0xf5, 0xff}
	colDim       = color.RGBA{0x5a, 0x54, 0x70, 0xff}
	colDimmer    = color.RGBA{0x38, 0x34, 0x4c, 0xff}
	colGreen     = color.RGBA{0x34, 0xd3, 0x99, 0xff}
	colAmber     = color.RGBA{0xfb, 0xbf, 0x24, 0xff}
	colRed       = color.RGBA{0xf8, 0x71, 0x71, 0xff}
)

// fade scales a color's RGB toward the background by t in [0,1].
func fade(c color.RGBA, t float32) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	lerp := func(a, b uint8) uint8 {
		return uint8(float32(a) + (float32(b)-float32(a))*t)
	}
	return color.RGBA{
		lerp(c.R, colBG.R),
		lerp(c.G, colBG.G),
		lerp(c.B, colBG.B),
		0xff,
	}
}
