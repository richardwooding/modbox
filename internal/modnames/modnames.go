// Package modnames extracts instrument/sample names straight from tracker
// module bytes. gotracker/playback doesn't expose them, but every format
// stores names at documented offsets — and scene artists traditionally use
// them as a message board, so the panel is worth the parsing.
//
// All readers are best-effort: any structural surprise returns what was
// gathered so far (or nil), never an error — the UI falls back to numbers.
package modnames

import (
	"encoding/binary"
	"strings"
)

// Names returns the instrument (or sample) names for a MOD/S3M/XM/IT module,
// index 0 = instrument 1. Unknown formats yield nil.
func Names(data []byte) []string {
	switch {
	case len(data) >= 4 && string(data[:4]) == "IMPM":
		return itNames(data)
	case len(data) >= 17 && string(data[:17]) == "Extended Module: ":
		return xmNames(data)
	case len(data) >= 48 && string(data[44:48]) == "SCRM":
		return s3mNames(data)
	case len(data) >= 1084:
		return modNames(data)
	default:
		return nil
	}
}

func clean(b []byte) string {
	s := strings.TrimRight(string(b), "\x00 ")
	// Strip any embedded NULs a sloppy tracker left behind.
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return s
}

func u16(data []byte, off int) (int, bool) {
	if off < 0 || off+2 > len(data) {
		return 0, false
	}
	return int(binary.LittleEndian.Uint16(data[off:])), true
}

func u32(data []byte, off int) (int, bool) {
	if off < 0 || off+4 > len(data) {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(data[off:])
	if v > 1<<30 {
		return 0, false
	}
	return int(v), true
}

// modNames: 31 sample headers of 30 bytes each, starting at offset 20;
// the first 22 bytes of each are the name.
func modNames(data []byte) []string {
	names := make([]string, 0, 31)
	for i := range 31 {
		off := 20 + i*30
		if off+22 > len(data) {
			break
		}
		names = append(names, clean(data[off:off+22]))
	}
	return names
}

// xmNames walks the pattern headers to reach the instrument blocks.
func xmNames(data []byte) []string {
	hsize, ok := u32(data, 60)
	if !ok {
		return nil
	}
	npat, ok1 := u16(data, 70)
	nins, ok2 := u16(data, 72)
	if !ok1 || !ok2 || nins > 256 {
		return nil
	}
	off := 60 + hsize
	for range npat {
		phl, ok1 := u32(data, off)
		psize, ok2 := u16(data, off+7)
		if !ok1 || !ok2 {
			return nil
		}
		off += phl + psize
	}
	var names []string
	for range nins {
		isize, ok := u32(data, off)
		if !ok || off+29 > len(data) {
			return names
		}
		names = append(names, clean(data[off+4:off+26]))
		nsamp, ok := u16(data, off+27)
		if !ok {
			return names
		}
		if nsamp == 0 {
			off += isize
			continue
		}
		// 40-byte sample headers follow the instrument header, then the
		// sample data itself.
		total := 0
		soff := off + isize
		for range nsamp {
			slen, ok := u32(data, soff)
			if !ok {
				return names
			}
			total += slen
			soff += 40
		}
		off = soff + total
	}
	return names
}

// s3mNames follows the instrument parapointers; the sample name lives at
// offset 48 (28 bytes) of each instrument block.
func s3mNames(data []byte) []string {
	ordNum, ok1 := u16(data, 32)
	insNum, ok2 := u16(data, 34)
	if !ok1 || !ok2 || insNum > 256 {
		return nil
	}
	ptrs := 96 + ordNum
	var names []string
	for i := range insNum {
		para, ok := u16(data, ptrs+i*2)
		if !ok {
			return names
		}
		off := para * 16
		if off+76 > len(data) {
			return names
		}
		names = append(names, clean(data[off+48:off+76]))
	}
	return names
}

// itNames prefers instrument names; sample-mode songs (no instruments) fall
// back to sample names. Both live 26 bytes into their blocks... roughly.
func itNames(data []byte) []string {
	ordNum, ok1 := u16(data, 0x20)
	insNum, ok2 := u16(data, 0x22)
	smpNum, ok3 := u16(data, 0x24)
	if !ok1 || !ok2 || !ok3 {
		return nil
	}
	table := 0xC0 + ordNum
	if insNum > 0 && insNum <= 256 {
		return itBlockNames(data, table, insNum, 0x20, 26)
	}
	if smpNum > 0 && smpNum <= 256 {
		return itBlockNames(data, table+insNum*4, smpNum, 0x14, 26)
	}
	return nil
}

func itBlockNames(data []byte, table, count, nameOff, nameLen int) []string {
	var names []string
	for i := range count {
		off, ok := u32(data, table+i*4)
		if !ok || off == 0 || off+nameOff+nameLen > len(data) {
			return names
		}
		names = append(names, clean(data[off+nameOff:off+nameOff+nameLen]))
	}
	return names
}
