// Package modules embeds the bundled demo songs. Every file here must have a
// clear license (Creative Commons or public domain) with attribution recorded
// both in NOTICE.md and in the Demo metadata rendered by the UI.
package modules

// Demo is one bundled module with its attribution.
type Demo struct {
	Title   string
	Artist  string
	License string
	Source  string
	Data    []byte
}

// demos is populated in embed.go once licensed modules are vendored.
var demos []Demo

// Demos lists the bundled songs shown on the drop scene.
func Demos() []Demo { return demos }
