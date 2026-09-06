// Package pixels converts a decoded XISF/FITS image into an 8-bit image,
// applying a linear per-image min/max stretch to the full sample range.
//
// Port of the Rust pixels.rs.
package pixels

import (
	"encoding/binary"
	"fmt"
	"image"
	"math"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
)

// ToImage stretches raw to 8 bits. Mono -> *image.Gray, RGB -> *image.RGBA
// (opaque).
func ToImage(raw imageio.Raw) (image.Image, error) {
	if raw.Channels != 1 && raw.Channels != 3 {
		return nil, fmt.Errorf("unsupported channel count %d (only mono and RGB are handled)", raw.Channels)
	}
	w, h, c := raw.Width, raw.Height, raw.Channels
	count := w * h * c

	samples, err := decodeSamples(raw, count)
	if err != nil {
		return nil, err
	}

	// Planar (channel-major) -> interleaved (pixel-major).
	if raw.Planar && c > 1 {
		inter := make([]float64, count)
		plane := w * h
		for ch := 0; ch < c; ch++ {
			for i := 0; i < plane; i++ {
				inter[i*c+ch] = samples[ch*plane+i]
			}
		}
		samples = inter
	}

	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, v := range samples {
		if math.IsNaN(v) {
			continue
		}
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	px := make([]byte, count)
	if !math.IsInf(minV, 0) && !math.IsInf(maxV, 0) && maxV > minV {
		scale := 255.0 / (maxV - minV)
		for i, v := range samples {
			if math.IsNaN(v) {
				continue
			}
			s := (v - minV) * scale
			switch {
			case s <= 0:
				px[i] = 0
			case s >= 255:
				px[i] = 255
			default:
				px[i] = byte(s + 0.5)
			}
		}
	}

	if c == 1 {
		img := image.NewGray(image.Rect(0, 0, w, h))
		copy(img.Pix, px)
		return img, nil
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4+0] = px[i*3+0]
		img.Pix[i*4+1] = px[i*3+1]
		img.Pix[i*4+2] = px[i*3+2]
		img.Pix[i*4+3] = 255
	}
	return img, nil
}

func decodeSamples(raw imageio.Raw, count int) ([]float64, error) {
	bps := raw.Format.BytesPerSample()
	if len(raw.Data) < count*bps {
		return nil, fmt.Errorf("pixel data too small: got %d bytes, expected %d", len(raw.Data), count*bps)
	}
	var bo binary.ByteOrder = binary.LittleEndian
	if raw.BigEndian {
		bo = binary.BigEndian
	}
	out := make([]float64, count)
	for i := 0; i < count; i++ {
		b := raw.Data[i*bps:]
		switch raw.Format {
		case imageio.U8:
			out[i] = float64(b[0])
		case imageio.U16:
			out[i] = float64(bo.Uint16(b))
		case imageio.U32:
			out[i] = float64(bo.Uint32(b))
		case imageio.U64:
			out[i] = float64(bo.Uint64(b))
		case imageio.F32:
			out[i] = float64(math.Float32frombits(bo.Uint32(b)))
		case imageio.F64:
			out[i] = math.Float64frombits(bo.Uint64(b))
		}
	}
	return out, nil
}
