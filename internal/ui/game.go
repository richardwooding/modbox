// Package ui is the Ebitengine front end: a drop/landing scene and the
// player scene with pattern grid, oscilloscopes, and VU meters.
package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	// Logical canvas size; ebiten scales to the window / browser canvas.
	W = 960
	H = 600
)

// scene is one screen of the app.
type scene interface {
	Update(g *Game) error
	Draw(dst *ebiten.Image)
}

// Game is the ebiten.Game; it owns the audio context and current scene.
type Game struct {
	audioCtx *audio.Context
	scene    scene
}

func NewGame() *Game {
	g := &Game{audioCtx: audio.NewContext(44100)}
	g.scene = newDropScene()
	return g
}

func (g *Game) Update() error              { return g.scene.Update(g) }
func (g *Game) Draw(dst *ebiten.Image)     { dst.Fill(colBG); g.scene.Draw(dst) }
func (g *Game) Layout(_, _ int) (int, int) { return W, H }
