// Package fits is a minimal FITS reader: the image in the primary HDU of a
// .fits / .fit / .fts file. BITPIX 8/16/32/64/-32/-64; NAXIS 2 (mono) or 3
// with NAXIS3 = 1 or 3; BZERO/BSCALE; ROWORDER.
//
// Port of the Rust fits.rs.
package fits

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
	"github.com/peterbuitho/AstroGoPNG/internal/wcs"
)

const (
	block = 2880
	card  = 80
)

// Parse decodes an in-memory FITS file.
func Parse(b []byte) (imageio.Raw, error) {
	var raw imageio.Raw
	if len(b) < block || !strings.HasPrefix(string(b[:9]), "SIMPLE  =") {
		return raw, fmt.Errorf("not a FITS file (missing SIMPLE card)")
	}
	h, err := parseHeaderCards(b)
	if err != nil {
		return raw, err
	}
	if v, _ := h.boolean("SIMPLE"); !v {
		return raw, fmt.Errorf("non-standard FITS file (SIMPLE is not T)")
	}
	bitpix, ok := h.int("BITPIX")
	if !ok {
		return raw, fmt.Errorf("BITPIX missing")
	}
	naxis, ok := h.int("NAXIS")
	if !ok {
		return raw, fmt.Errorf("NAXIS missing")
	}

	var width, height, channels int
	switch naxis {
	case 0:
		return raw, fmt.Errorf("primary HDU has no image data (NAXIS = 0); images in extensions are not supported")
	case 2:
		if width, err = h.axis(1); err != nil {
			return raw, err
		}
		if height, err = h.axis(2); err != nil {
			return raw, err
		}
		channels = 1
	case 3:
		c, err := h.axis(3)
		if err != nil {
			return raw, err
		}
		if c != 1 && c != 3 {
			return raw, fmt.Errorf("unsupported NAXIS3 = %d (only 1 or 3 channel images are handled)", c)
		}
		if width, err = h.axis(1); err != nil {
			return raw, err
		}
		if height, err = h.axis(2); err != nil {
			return raw, err
		}
		channels = c
	default:
		return raw, fmt.Errorf("unsupported NAXIS = %d (only 2-D images and 3-plane RGB are handled)", naxis)
	}
	if width == 0 || height == 0 {
		return raw, fmt.Errorf("image has zero size")
	}

	var bytesPerSample int
	switch bitpix {
	case 8:
		bytesPerSample = 1
	case 16:
		bytesPerSample = 2
	case 32, -32:
		bytesPerSample = 4
	case 64, -64:
		bytesPerSample = 8
	default:
		return raw, fmt.Errorf("unsupported BITPIX = %d", bitpix)
	}

	count := width * height * channels
	dataLen := count * bytesPerSample
	if h.dataOffset+dataLen > len(b) {
		return raw, fmt.Errorf("pixel data truncated: need %d bytes after the header, file has %d", dataLen, len(b)-h.dataOffset)
	}
	data := b[h.dataOffset : h.dataOffset+dataLen]

	bzero := h.floatOr("BZERO", 0)
	bscale := h.floatOr("BSCALE", 1)
	scaled := bzero != 0 || bscale != 1

	bottomUp := true
	if s, ok := h.str("ROWORDER"); ok && strings.EqualFold(strings.TrimSpace(s), "TOP-DOWN") {
		bottomUp = false
	}

	w, hh, c := width, height, channels
	plane := w * hh

	if bitpix == 8 && !scaled {
		raw.Format = imageio.U8
		raw.Data = flipRows(data, w, hh, c, 1, bottomUp)
	} else if bitpix == -32 && !scaled {
		raw.Format = imageio.F32
		raw.Data = flipRows(data, w, hh, c, 4, bottomUp)
	} else if bitpix == -64 && !scaled {
		raw.Format = imageio.F64
		raw.Data = flipRows(data, w, hh, c, 8, bottomUp)
	} else {
		raw.Format = imageio.F32
		out := make([]byte, count*4)
		for ch := 0; ch < c; ch++ {
			for y := 0; y < hh; y++ {
				dstY := y
				if bottomUp {
					dstY = hh - 1 - y
				}
				for x := 0; x < w; x++ {
					i := ch*plane + y*w + x
					v := float32(bzero + bscale*readSample(bitpix, data[i*bytesPerSample:]))
					o := (ch*plane + dstY*w + x) * 4
					binary.BigEndian.PutUint32(out[o:], math.Float32bits(v))
				}
			}
		}
		raw.Data = out
	}

	raw.Width, raw.Height, raw.Channels = width, height, channels
	raw.Planar = true
	raw.BigEndian = true
	if s, ok := h.str("OBJECT"); ok {
		raw.Object = strings.TrimSpace(s)
	}
	if c, ok := wcs.FromKeywords(h.value, width, height); ok {
		raw.Coords, raw.HasCoords = c, true
	}
	return raw, nil
}

func readSample(bitpix int64, b []byte) float64 {
	switch bitpix {
	case 8:
		return float64(b[0])
	case 16:
		return float64(int16(binary.BigEndian.Uint16(b)))
	case 32:
		return float64(int32(binary.BigEndian.Uint32(b)))
	case 64:
		return float64(int64(binary.BigEndian.Uint64(b)))
	case -32:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case -64:
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	}
	return 0
}

func flipRows(data []byte, w, h, c, bps int, flip bool) []byte {
	if !flip {
		return append([]byte(nil), data...)
	}
	row := w * bps
	plane := row * h
	out := make([]byte, len(data))
	for ch := 0; ch < c; ch++ {
		for y := 0; y < h; y++ {
			src := ch*plane + y*row
			dst := ch*plane + (h-1-y)*row
			copy(out[dst:dst+row], data[src:src+row])
		}
	}
	return out
}

// --- header ---------------------------------------------------------

type fitsHeader struct {
	cards      map[string]string
	order      []string
	dataOffset int
}

func parseHeaderCards(b []byte) (*fitsHeader, error) {
	h := &fitsHeader{cards: map[string]string{}}
	pos := 0
	for {
		if pos+card > len(b) {
			return nil, fmt.Errorf("header has no END card")
		}
		c := b[pos : pos+card]
		pos += card
		key := strings.TrimRight(string(c[:8]), " ")
		if key == "END" {
			break
		}
		if c[8] == '=' && c[9] == ' ' {
			val := stripComment(string(c[10:]))
			if _, dup := h.cards[key]; !dup {
				h.order = append(h.order, key)
			}
			h.cards[key] = val
		}
	}
	h.dataOffset = ((pos + block - 1) / block) * block
	return h, nil
}

func (h *fitsHeader) get(key string) (string, bool) { v, ok := h.cards[key]; return v, ok }

func (h *fitsHeader) boolean(key string) (bool, bool) {
	v, ok := h.get(key)
	if !ok {
		return false, false
	}
	switch v {
	case "T":
		return true, true
	case "F":
		return false, true
	}
	return false, false
}

func (h *fitsHeader) int(key string) (int64, bool) {
	v, ok := h.get(key)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n, err == nil
}

func (h *fitsHeader) floatOr(key string, def float64) float64 {
	v, ok := h.get(key)
	if !ok {
		return def
	}
	f, ok := wcs.ParseNumber(v)
	if !ok {
		return def
	}
	return f
}

// str returns a quoted string value with the quotes removed.
func (h *fitsHeader) str(key string) (string, bool) {
	v, ok := h.get(key)
	if !ok || len(v) == 0 || v[0] != '\'' {
		return "", false
	}
	inner := v[1:]
	if i := strings.LastIndexByte(inner, '\''); i >= 0 {
		inner = inner[:i]
	}
	return strings.TrimRight(strings.ReplaceAll(inner, "''", "'"), " "), true
}

// value returns any value as text: strings unquoted, numbers as written.
func (h *fitsHeader) value(key string) (string, bool) {
	v, ok := h.get(key)
	if !ok {
		return "", false
	}
	if len(v) > 0 && v[0] == '\'' {
		return h.str(key)
	}
	t := strings.TrimSpace(v)
	if t == "" {
		return "", false
	}
	return t, true
}

func (h *fitsHeader) axis(n int) (int, error) {
	key := "NAXIS" + strconv.Itoa(n)
	v, ok := h.int(key)
	if !ok {
		return 0, fmt.Errorf("%s missing", key)
	}
	if v < 0 {
		return 0, fmt.Errorf("%s = %d is negative", key, v)
	}
	return int(v), nil
}

func stripComment(field string) string {
	field = strings.TrimSpace(field)
	if strings.HasPrefix(field, "'") {
		b := field[1:]
		i := 0
		for i < len(b) {
			if b[i] == '\'' {
				if i+1 < len(b) && b[i+1] == '\'' {
					i += 2
					continue
				}
				return field[:i+2]
			}
			i++
		}
		return field
	}
	if i := strings.IndexByte(field, '/'); i >= 0 {
		return strings.TrimSpace(field[:i])
	}
	return field
}
