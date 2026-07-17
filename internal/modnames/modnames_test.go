package modnames_test

import (
	"testing"

	"github.com/richardwooding/modbox/internal/modnames"
	"github.com/richardwooding/modbox/internal/modules"
)

// The Drozerix demos carry his traditional signature in the instrument
// names — verified against a manual walk of the XM structure.
func TestXMNamesFromDemos(t *testing.T) {
	demos := modules.Demos()
	if len(demos) == 0 {
		t.Fatal("no demos")
	}
	names := modnames.Names(demos[0].Data) // Bit Loader
	if len(names) != 4 {
		t.Fatalf("Bit Loader: got %d names, want 4: %q", len(names), names)
	}
	want := []string{"Sampled & Tracked By:", "Drozerix", "----------------------", "Drozerix@gmail.com"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("name[%d] = %q, want %q", i, names[i], w)
		}
	}
}

func TestNamesToleratesGarbage(t *testing.T) {
	for _, data := range [][]byte{
		nil,
		[]byte("hello"),
		[]byte("Extended Module: truncated"),
		make([]byte, 2000), // MOD-sized zeros
	} {
		_ = modnames.Names(data) // must not panic
	}
}
