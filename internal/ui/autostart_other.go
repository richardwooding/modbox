//go:build !js

package ui

// autostartDemo is a no-op outside the browser.
func autostartDemo() int { return -1 }

// autostartFX is a no-op outside the browser.
func autostartFX() bool { return false }
