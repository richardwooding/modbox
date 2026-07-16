// modbox plays classic tracker music (MOD/S3M/XM/IT) with a FastTracker-
// flavored visualizer — pattern scroller, per-channel oscilloscopes, VU
// meters. One Go codebase: a native window for development, WebAssembly in
// the browser for the real thing.
package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/richardwooding/modbox/internal/ui"
)

func main() {
	ebiten.SetWindowSize(ui.W, ui.H)
	ebiten.SetWindowTitle("modbox")
	if err := ebiten.RunGame(ui.NewGame()); err != nil {
		log.Fatal(err)
	}
}
