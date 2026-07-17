//go:build !js

package ui

// Native builds have no file dialog; drag-and-drop covers development use.
func canPickFiles() bool                     { return false }
func openFilePicker()                        {}
func takePickedFile() ([]byte, string, bool) { return nil, "", false }
