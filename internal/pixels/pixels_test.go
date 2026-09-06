package pixels

import (
	"image"
	"testing"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
)

func TestLinearStretch(t *testing.T) {
	// 2x1 mono UInt16 big-endian: 1000 and 5000 -> 0 and 255.
	raw := imageio.Raw{
		Width: 2, Height: 1, Channels: 1, Format: imageio.U16,
		Planar: true, BigEndian: true,
		Data: []byte{0x03, 0xE8, 0x13, 0x88},
	}
	img, err := ToImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := img.(*image.Gray)
	if !ok {
		t.Fatalf("want *image.Gray, got %T", img)
	}
	if g.Pix[0] != 0 || g.Pix[1] != 255 {
		t.Errorf("pixels = %v", g.Pix)
	}
}
