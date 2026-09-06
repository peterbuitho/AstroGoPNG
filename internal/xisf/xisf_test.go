package xisf

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
)

func TestLZ4RoundTrip(t *testing.T) {
	// 1 literal 'a', then match offset=1 len=4 (token low nibble 0 -> +4).
	src := []byte{0x10, 'a', 0x01, 0x00}
	dst := make([]byte, 5)
	if err := lz4DecompressBlock(src, dst); err != nil {
		t.Fatal(err)
	}
	if string(dst) != "aaaaa" {
		t.Errorf("got %q", dst)
	}
}

func TestUnshuffle(t *testing.T) {
	orig := []byte{0x11, 0x22, 0x33, 0x44, 0xAA, 0xBB, 0xCC, 0xDD}
	const item, items = 2, 4
	shuffled := make([]byte, 8)
	p := 0
	for b := 0; b < item; b++ {
		for i := 0; i < items; i++ {
			shuffled[p] = orig[i*item+b]
			p++
		}
	}
	back := unshuffle(shuffled, item)
	for i := range orig {
		if back[i] != orig[i] {
			t.Fatalf("unshuffle mismatch at %d: %v", i, back)
		}
	}
}

func TestParseAttachment(t *testing.T) {
	data := []byte{10, 20, 30, 40, 200, 210, 220, 230}
	const hdr = 512
	pos := 16 + hdr
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<xisf version="1.0">`+
		`<Image geometry="4:2:1" sampleFormat="UInt8" location="attachment:%d:8">`+
		`<FITSKeyword name="OBJECT" value="XTest"/>`+
		`</Image></xisf>`, pos)
	blob := append([]byte("XISF0100"), make([]byte, 8)...)
	binary.LittleEndian.PutUint32(blob[8:], hdr)
	header := make([]byte, hdr)
	copy(header, xml)
	blob = append(blob, header...)
	blob = append(blob, data...)

	raw, err := Parse(blob)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Width != 4 || raw.Height != 2 || raw.Channels != 1 || raw.Format != imageio.U8 {
		t.Fatalf("raw = %+v", raw)
	}
	if raw.Object != "XTest" {
		t.Errorf("object = %q", raw.Object)
	}
	if raw.Data[0] != 10 || raw.Data[7] != 230 {
		t.Errorf("payload = %v", raw.Data)
	}
}
