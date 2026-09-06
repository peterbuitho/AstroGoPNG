// Package xisf is a minimal reader for the monolithic XISF 1.0 file format,
// sufficient to pull the first image out of a PixInsight / N.I.N.A. file.
//
// Port of the Rust xisf.rs.
package xisf

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
	"github.com/peterbuitho/AstroGoPNG/internal/wcs"
)

var signature = []byte("XISF0100")

// Parse decodes the first <Image> in an in-memory XISF file.
func Parse(file []byte) (imageio.Raw, error) {
	var raw imageio.Raw
	if len(file) < 16 || !bytes.Equal(file[:8], signature) {
		return raw, fmt.Errorf("not an XISF 1.0 file (bad signature)")
	}
	headerLen := int(binary.LittleEndian.Uint32(file[8:12]))
	const xmlStart = 16
	xmlEnd := xmlStart + headerLen
	if len(file) < xmlEnd {
		return raw, fmt.Errorf("truncated XISF header")
	}
	xmlBytes := bytes.TrimRight(file[xmlStart:xmlEnd], "\x00")

	hdr, err := parseHeader(xmlBytes)
	if err != nil {
		return raw, fmt.Errorf("invalid XISF XML header: %w", err)
	}
	if hdr.image == nil {
		return raw, fmt.Errorf("no <Image> element in XISF header")
	}
	img := hdr.image

	get := func(k string) (string, bool) {
		if v, ok := hdr.fitsKeyword(k); ok {
			return v, true
		}
		switch k {
		case "RA":
			return hdr.property("Observation:Center:RA")
		case "DEC":
			return hdr.property("Observation:Center:Dec")
		}
		return "", false
	}

	raw.Object, _ = hdr.fitsKeyword("OBJECT")
	if raw.Object == "" {
		raw.Object, _ = hdr.property("Observation:Object:Name")
	}

	// --- geometry -------------------------------------------------------
	geometry := img.attr("geometry")
	if geometry == "" {
		return raw, fmt.Errorf("<Image> missing geometry attribute")
	}
	var geo []int64
	for _, part := range strings.Split(geometry, ":") {
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return raw, fmt.Errorf("invalid geometry %q", geometry)
		}
		geo = append(geo, n)
	}
	if len(geo) < 3 {
		return raw, fmt.Errorf("unsupported geometry %q (expected w:h:channels)", geometry)
	}
	gw, gh, gc := geo[0], geo[1], geo[len(geo)-1]
	if gw <= 0 || gh <= 0 || gc <= 0 {
		return raw, fmt.Errorf("invalid geometry %q", geometry)
	}
	raw.Width, raw.Height, raw.Channels = int(gw), int(gh), int(gc)

	if c, ok := wcs.FromKeywords(get, raw.Width, raw.Height); ok {
		raw.Coords, raw.HasCoords = c, true
	}

	// --- sample format ------------------------------------------------
	switch img.attr("sampleFormat") {
	case "", "UInt16":
		raw.Format = imageio.U16
	case "UInt8":
		raw.Format = imageio.U8
	case "UInt32":
		raw.Format = imageio.U32
	case "UInt64":
		raw.Format = imageio.U64
	case "Float32":
		raw.Format = imageio.F32
	case "Float64":
		raw.Format = imageio.F64
	default:
		return raw, fmt.Errorf("unsupported sampleFormat %q", img.attr("sampleFormat"))
	}

	raw.Planar = !strings.EqualFold(img.attr("pixelStorage"), "Normal")
	raw.BigEndian = strings.EqualFold(img.attr("byteOrder"), "big")

	expectedBytes := int64(raw.Width) * int64(raw.Height) * int64(raw.Channels) * int64(raw.Format.BytesPerSample())

	// --- locate + decode payload -----------------------------------
	location := img.attr("location")
	if location == "" {
		return raw, fmt.Errorf("<Image> missing location attribute")
	}
	compression := img.attr("compression")

	loc := strings.Split(location, ":")
	var payload []byte
	switch loc[0] {
	case "attachment":
		if len(loc) < 3 {
			return raw, fmt.Errorf("malformed attachment location %q", location)
		}
		pos, err1 := strconv.Atoi(loc[1])
		size, err2 := strconv.Atoi(loc[2])
		if err1 != nil || err2 != nil {
			return raw, fmt.Errorf("malformed attachment location %q", location)
		}
		end := pos + size
		if pos < 0 || end < pos || end > len(file) {
			return raw, fmt.Errorf("attachment extends past end of file")
		}
		payload = append([]byte(nil), file[pos:end]...)
	case "embedded":
		if img.data == nil {
			return raw, fmt.Errorf("embedded location but no <Data> child")
		}
		if compression == "" {
			compression = img.data.attr("compression")
		}
		enc := img.data.attr("encoding")
		if enc == "" {
			enc = "base64"
		}
		payload, err = decodeText(img.data.text, enc)
		if err != nil {
			return raw, err
		}
	case "inline":
		enc := "base64"
		if len(loc) > 1 {
			enc = loc[1]
		}
		payload, err = decodeText(img.text, enc)
		if err != nil {
			return raw, err
		}
	default:
		return raw, fmt.Errorf("unsupported location kind %q", loc[0])
	}

	// --- decompress + un-shuffle -----------------------------------
	if compression != "" {
		payload, err = decompress(payload, compression)
		if err != nil {
			return raw, err
		}
	}
	if int64(len(payload)) < expectedBytes {
		return raw, fmt.Errorf("pixel data too small: got %d bytes, expected %d", len(payload), expectedBytes)
	}
	raw.Data = payload
	return raw, nil
}

func decodeText(text, encoding string) ([]byte, error) {
	var b strings.Builder
	for _, r := range text {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	switch strings.ToLower(encoding) {
	case "base64":
		out, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 data: %w", err)
		}
		return out, nil
	case "hex":
		out, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid hex data: %w", err)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported data encoding %q", encoding)
}

func decompress(input []byte, spec string) ([]byte, error) {
	// grammar: codec[+sh]:uncompressedSize[:shuffleItemSize]
	parts := strings.Split(spec, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed compression spec %q", spec)
	}
	codecSpec := strings.ToLower(parts[0])
	uncompressed, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed compression spec %q", spec)
	}
	shuffled := strings.HasSuffix(codecSpec, "+sh")
	codec := strings.TrimSuffix(codecSpec, "+sh")
	itemSize := 1
	if len(parts) > 2 {
		if itemSize, err = strconv.Atoi(parts[2]); err != nil {
			return nil, fmt.Errorf("malformed compression spec %q", spec)
		}
	}

	var out []byte
	switch codec {
	case "lz4", "lz4hc":
		out = make([]byte, uncompressed)
		if err := lz4DecompressBlock(input, out); err != nil {
			return nil, err
		}
	case "zlib":
		zr, err := zlib.NewReader(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("zlib decode failed: %w", err)
		}
		out, err = io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("zlib decode failed: %w", err)
		}
	case "zstd":
		zr, err := zstd.NewReader(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("zstd decode failed: %w", err)
		}
		defer zr.Close()
		out, err = io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("zstd decode failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported compression codec %q", codec)
	}

	if len(out) != uncompressed {
		return nil, fmt.Errorf("%s decode produced %d bytes, expected %d", codec, len(out), uncompressed)
	}
	if shuffled && itemSize > 1 {
		out = unshuffle(out, itemSize)
	}
	return out, nil
}

// unshuffle reverses the XISF byte-shuffle.
func unshuffle(in []byte, itemSize int) []byte {
	items := len(in) / itemSize
	out := make([]byte, len(in))
	p := 0
	for b := 0; b < itemSize; b++ {
		for i := 0; i < items; i++ {
			out[i*itemSize+b] = in[p]
			p++
		}
	}
	for k := items * itemSize; k < len(in); k++ {
		out[k] = in[k]
	}
	return out
}

// --- tiny XML model ---------------------------------------------------

type element struct {
	name  string
	attrs []xml.Attr
	text  string
	data  *element // the <Data> child of an <Image>, if any
}

func (e *element) attr(name string) string {
	for _, a := range e.attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

type header struct {
	image      *element
	fitsKW     []*element // FITSKeyword children of the first image
	properties []*element // Property elements anywhere
}

func (h *header) fitsKeyword(key string) (string, bool) {
	for _, kw := range h.fitsKW {
		if strings.EqualFold(strings.TrimSpace(kw.attr("name")), key) {
			v := strings.TrimSpace(strings.Trim(strings.TrimSpace(kw.attr("value")), "'"))
			if v == "" {
				return "", false
			}
			return v, true
		}
	}
	return "", false
}

func (h *header) property(id string) (string, bool) {
	for _, p := range h.properties {
		if p.attr("id") != id {
			continue
		}
		if t := strings.TrimSpace(p.text); t != "" {
			return t, true
		}
		if v := p.attr("value"); v != "" {
			return v, true
		}
		return "", false
	}
	return "", false
}

func parseHeader(src []byte) (*header, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	h := &header{}
	var stack []*element
	var imageDepth = -1

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := &element{name: t.Name.Local, attrs: append([]xml.Attr(nil), t.Attr...)}
			switch el.name {
			case "Image":
				if h.image == nil {
					h.image = el
					imageDepth = len(stack)
				}
			case "FITSKeyword":
				if h.image != nil && len(stack) == imageDepth+1 && stack[len(stack)-1] == h.image {
					h.fitsKW = append(h.fitsKW, el)
				}
			case "Data":
				if h.image != nil && len(stack) == imageDepth+1 && stack[len(stack)-1] == h.image {
					h.image.data = el
				}
			case "Property":
				h.properties = append(h.properties, el)
			}
			stack = append(stack, el)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return h, nil
}
