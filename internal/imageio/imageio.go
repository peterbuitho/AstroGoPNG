// Package imageio holds the shared decoded-image type produced by the XISF and
// FITS readers, plus PNG encode/decode and the 4K resize.
package imageio

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"golang.org/x/image/draw"

	"github.com/peterbuitho/AstroGoPNG/internal/wcs"
)

// SampleFormat is the on-disk numeric type of a raw sample.
type SampleFormat int

const (
	U8 SampleFormat = iota
	U16
	U32
	U64
	F32
	F64
)

func (f SampleFormat) BytesPerSample() int {
	switch f {
	case U8:
		return 1
	case U16:
		return 2
	case U32, F32:
		return 4
	case U64, F64:
		return 8
	}
	return 0
}

// Raw is the decoded pixel payload of the first image in an XISF or FITS file,
// plus the metadata needed to interpret the bytes.
type Raw struct {
	Width, Height, Channels int
	Format                  SampleFormat
	// Planar = channel-major storage; otherwise interleaved ("Normal").
	Planar bool
	// BigEndian is true when samples are stored big-endian.
	BigEndian bool
	// Data holds the raw, decompressed, un-shuffled sample bytes.
	Data []byte
	// Object is the target name from the header, if present.
	Object string
	// Coords is the image centre from the plate solution or mount target.
	Coords    wcs.SkyCoords
	HasCoords bool
}

// Encode writes img as a PNG.
func Encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode reads a PNG.
func Decode(b []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(b))
}

// ResizeToFill scales img (aspect ratio kept) to cover tw x th, then
// centre-crops the overflow to exactly tw x th. Uses a Catmull-Rom kernel,
// close to the Rust code's Lanczos3.
func ResizeToFill(src image.Image, tw, th int) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	scale := max64(float64(tw)/float64(sw), float64(th)/float64(sh))
	rw := int(float64(sw)*scale + 0.5)
	rh := int(float64(sh)*scale + 0.5)
	if rw < tw {
		rw = tw
	}
	if rh < th {
		rh = th
	}

	var scaled draw.Image
	switch src.(type) {
	case *image.Gray:
		scaled = image.NewGray(image.Rect(0, 0, rw, rh))
	default:
		scaled = image.NewRGBA(image.Rect(0, 0, rw, rh))
	}
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, sb, draw.Over, nil)

	ox := (rw - tw) / 2
	oy := (rh - th) / 2
	crop := image.Rect(ox, oy, ox+tw, oy+th)

	var out draw.Image
	switch scaled.(type) {
	case *image.Gray:
		out = image.NewGray(image.Rect(0, 0, tw, th))
	default:
		out = image.NewRGBA(image.Rect(0, 0, tw, th))
	}
	draw.Draw(out, out.Bounds(), scaled, crop.Min, draw.Src)
	return out
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// String makes Raw a little friendlier in error messages.
func (r Raw) String() string {
	return fmt.Sprintf("%dx%dx%d %v", r.Width, r.Height, r.Channels, r.Format)
}
