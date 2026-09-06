// Package stamp draws the bottom-right corner label onto a resized image:
// a title line and an optional smaller line underneath, right-aligned, white
// over a soft drop shadow, each line auto-shrunk to fit the margin.
//
// Port of the Rust post.rs (the resize itself lives in package imageio).
package stamp

import (
	_ "embed"
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
)

const (
	TargetWidth  = 3840
	TargetHeight = 2160

	titlePx      = 48.0
	subtitlePx   = 30.0
	lineGap      = 14.0
	marginRight  = 60.0
	marginBottom = 120.0
	shadowOffset = 2.0
	shadowAlpha  = 179 // 0.7 * 255
)

//go:embed dejavu.ttf
var bundledFont []byte

// BundledFontBytes is the embedded DejaVu Sans Condensed Bold.
func BundledFontBytes() []byte { return bundledFont }

// Label is what gets stamped: a title and an optional smaller subtitle.
type Label struct {
	Title    string
	Subtitle string
}

// Stamper holds the parsed label font; create once and reuse.
type Stamper struct {
	font *sfnt.Font
}

// Bundled uses the embedded font.
func Bundled() (*Stamper, error) { return New(bundledFont) }

// New uses a TrueType / OpenType font supplied as bytes.
func New(fontBytes []byte) (*Stamper, error) {
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	return &Stamper{font: f}, nil
}

// ResizeAndLabel scales img to cover 3840x2160, centre-crops, then draws label.
func (s *Stamper) ResizeAndLabel(img image.Image, label Label) (image.Image, error) {
	resized := imageio.ResizeToFill(img, TargetWidth, TargetHeight)
	dst, ok := resized.(draw.Image)
	if !ok {
		rgba := image.NewRGBA(resized.Bounds())
		draw.Draw(rgba, rgba.Bounds(), resized, resized.Bounds().Min, draw.Src)
		dst = rgba
	}
	if err := s.drawLabel(dst, label); err != nil {
		return nil, err
	}
	return dst, nil
}

func (s *Stamper) face(px float64) (font.Face, error) {
	return opentype.NewFace(s.font, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
}

func (s *Stamper) drawLabel(dst draw.Image, label Label) error {
	b := dst.Bounds()
	right := float64(b.Dx()) - marginRight
	bottom := float64(b.Dy()) - marginBottom
	maxWidth := float64(b.Dx()) - 2*marginRight

	titleBaseline := bottom

	if label.Subtitle != "" {
		subFace, err := s.face(subtitlePx)
		if err != nil {
			return err
		}
		m := subFace.Metrics()
		subBaseline := bottom - f2f(m.Descent)
		_ = subFace.Close()
		if err := s.drawLine(dst, label.Subtitle, subtitlePx, right, subBaseline, maxWidth); err != nil {
			return err
		}
		titleBaseline = subBaseline - f2f(m.Ascent) - lineGap
	} else {
		tf, err := s.face(titlePx)
		if err != nil {
			return err
		}
		titleBaseline = bottom - f2f(tf.Metrics().Descent)
		_ = tf.Close()
	}

	if label.Title != "" {
		return s.drawLine(dst, label.Title, titlePx, right, titleBaseline, maxWidth)
	}
	return nil
}

func (s *Stamper) drawLine(dst draw.Image, text string, px, right, baseline, maxWidth float64) error {
	face, err := s.face(px)
	if err != nil {
		return err
	}
	width := f2f(font.MeasureString(face, text))
	if width > maxWidth && width > 0 {
		_ = face.Close()
		px = px * maxWidth / width
		if face, err = s.face(px); err != nil {
			return err
		}
		width = f2f(font.MeasureString(face, text))
	}
	defer face.Close()

	x0 := right - width

	shadow := image.NewUniform(color.RGBA{0, 0, 0, shadowAlpha})
	white := image.NewUniform(color.RGBA{255, 255, 255, 255})

	drawAt := func(src image.Image, x, y float64) {
		d := font.Drawer{Dst: dst, Src: src, Face: face, Dot: fixed.Point26_6{
			X: fixed.Int26_6(x * 64),
			Y: fixed.Int26_6(y * 64),
		}}
		d.DrawString(text)
	}
	drawAt(shadow, x0+shadowOffset, baseline+shadowOffset)
	drawAt(white, x0, baseline)
	return nil
}

func f2f(v fixed.Int26_6) float64 { return float64(v) / 64 }
