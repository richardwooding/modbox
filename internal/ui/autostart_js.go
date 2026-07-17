//go:build js && wasm

package ui

import (
	"strconv"
	"strings"
	"syscall/js"
)

// autostartDemo reads ?demo=N from the page URL so links can deep-link a
// bundled song (and headless smoke tests can reach the player scene).
func autostartDemo() int {
	search := js.Global().Get("location").Get("search").String()
	for q := range strings.SplitSeq(strings.TrimPrefix(search, "?"), "&") {
		if v, ok := strings.CutPrefix(q, "demo="); ok {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				return n
			}
		}
	}
	return -1
}

// autostartFX reads ?fx=1 — deep-link straight into the spectacle view
// (fullscreen itself still waits for a gesture; browsers require one).
func autostartFX() bool {
	search := js.Global().Get("location").Get("search").String()
	return strings.Contains(search, "fx=1")
}
