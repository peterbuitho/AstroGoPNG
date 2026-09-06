// Package wcs derives sky coordinates from an image header: either the plate
// solution (WCS keywords, i.e. where the frame really points) or the mount's
// target coordinates. Used to cross-check the object name and to identify
// unnamed frames.
//
// Port of the Rust wcs.rs.
package wcs

import (
	"math"
	"strconv"
	"strings"
)

const deg2rad = math.Pi / 180

// SkyCoords is where the image is centred on the sky.
type SkyCoords struct {
	RADeg, DecDeg float64
	// FOVRadiusDeg is half the image diagonal in degrees when the pixel scale
	// is known; 0 (with FOVKnown false) otherwise.
	FOVRadiusDeg float64
	FOVKnown     bool
	// Solved is true when derived from a plate solution rather than the mount
	// target.
	Solved bool
}

// ToleranceDeg is how far a named object may sit from the image centre before
// we doubt the name.
func (c SkyCoords) ToleranceDeg() float64 {
	f := 0.0
	if c.FOVKnown {
		f = c.FOVRadiusDeg
	}
	return 2.0 + f
}

// SearchRadiusDeg is the radius for "what is at these coordinates?" searches.
func (c SkyCoords) SearchRadiusDeg() float64 {
	r := 1.0
	if c.FOVKnown {
		r = c.FOVRadiusDeg
	}
	return math.Max(0.25, math.Min(2.0, r))
}

func (c SkyCoords) SeparationTo(raDeg, decDeg float64) float64 {
	return SeparationDeg(c.RADeg, c.DecDeg, raDeg, decDeg)
}

// Getter returns the trimmed, unquoted value of a header keyword.
type Getter func(key string) (string, bool)

func (g Getter) num(key string) (float64, bool) {
	v, ok := g(key)
	if !ok {
		return 0, false
	}
	return ParseNumber(v)
}

// FromKeywords builds coordinates from header keywords. width/height are the
// image size in pixels.
func FromKeywords(get Getter, width, height int) (SkyCoords, bool) {
	w, h := float64(width), float64(height)

	// --- Plate solution ---------------------------------------------------
	if crval1, ok1 := get.num("CRVAL1"); ok1 {
		if crval2, ok2 := get.num("CRVAL2"); ok2 {
			ctypeOK := true
			if t, ok := get("CTYPE1"); ok {
				ctypeOK = strings.HasPrefix(strings.ToUpper(t), "RA")
			}
			if ctypeOK && crval1 >= 0 && crval1 <= 360 && crval2 >= -90 && crval2 <= 90 {
				var cd [4]float64
				haveCD := false
				if a, ok := get.num("CD1_1"); ok {
					if d, ok := get.num("CD2_2"); ok {
						b, _ := get.num("CD1_2")
						c, _ := get.num("CD2_1")
						cd = [4]float64{a, b, c, d}
						haveCD = true
					}
				}
				if !haveCD {
					if dx, ok := get.num("CDELT1"); ok {
						if dy, ok := get.num("CDELT2"); ok {
							rot := 0.0
							if r, ok := get.num("CROTA2"); ok {
								rot = r * deg2rad
							}
							cd = [4]float64{
								dx * math.Cos(rot),
								-dy * math.Sin(rot),
								dx * math.Sin(rot),
								dy * math.Cos(rot),
							}
							haveCD = true
						}
					}
				}

				ra, dec := crval1, crval2
				out := SkyCoords{RADeg: ra, DecDeg: dec, Solved: true}
				if haveCD {
					a, b, c, d := cd[0], cd[1], cd[2], cd[3]
					scale := math.Sqrt(math.Abs(a*d - b*c))
					if scale > 0 && scale < 1 {
						out.FOVRadiusDeg = 0.5 * scale * math.Hypot(w, h)
						out.FOVKnown = true
					}
					if crpix1, ok := get.num("CRPIX1"); ok {
						if crpix2, ok := get.num("CRPIX2"); ok {
							dxp := (w+1)/2 - crpix1
							dyp := (h+1)/2 - crpix2
							xi := a*dxp + b*dyp
							eta := c*dxp + d*dyp
							cosDec := math.Max(math.Cos(crval2*deg2rad), 1e-6)
							out.RADeg = remEuclid(crval1+xi/cosDec, 360)
							out.DecDeg = clamp(crval2+eta, -90, 90)
						}
					}
				}
				return out, true
			}
		}
	}

	// --- Mount / sequence target ----------------------------------------
	ra, okRA := firstAngle(get, true, "OBJCTRA")
	if !okRA {
		ra, okRA = firstAngle(get, false, "RA", "OBJRA")
	}
	dec, okDec := firstAngle(get, false, "OBJCTDEC", "DEC", "OBJDEC")
	if !okRA || !okDec {
		return SkyCoords{}, false
	}
	if ra < 0 || ra > 360 || dec < -90 || dec > 90 {
		return SkyCoords{}, false
	}

	out := SkyCoords{RADeg: ra, DecDeg: dec}
	if pix, ok := get.num("XPIXSZ"); ok && pix > 0 {
		if fl, ok := get.num("FOCALLEN"); ok && fl > 0 {
			bin := 1.0
			if b, ok := get.num("XBINNING"); ok && b >= 1 {
				bin = b
			}
			arcsecPerPx := 206.265 * pix * bin / fl
			out.FOVRadiusDeg = 0.5 * arcsecPerPx / 3600 * math.Hypot(w, h)
			out.FOVKnown = true
		}
	}
	return out, true
}

func firstAngle(get Getter, hours bool, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := get(k); ok {
			if a, ok := ParseAngle(v, hours); ok {
				return a, true
			}
		}
	}
	return 0, false
}

func remEuclid(v, m float64) float64 {
	r := math.Mod(v, m)
	if r < 0 {
		r += m
	}
	return r
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ParseNumber parses a plain number (FITS allows Fortran 'D' exponents).
func ParseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("D", "E", "d", "E").Replace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// ParseAngle parses an angle in degrees. Accepts decimal degrees or sexagesimal
// ("05 35 17.3", "05:35:17", "+41 16 08", "-05d23m28s"). hours means a
// sexagesimal (or bare decimal) value is in hours and must be scaled by 15.
func ParseAngle(s string, hours bool) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if !strings.ContainsAny(s, " :hdm") {
		v, ok := ParseNumber(s)
		if !ok {
			return 0, false
		}
		if hours {
			v *= 15
		}
		return v, true
	}

	negative := strings.HasPrefix(s, "-")
	body := strings.TrimLeft(s, "+-")
	fields := strings.FieldsFunc(body, func(r rune) bool {
		return strings.ContainsRune(" :hdms'\"", r)
	})
	if len(fields) == 0 || len(fields) > 3 {
		return 0, false
	}
	var parts [3]float64
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return 0, false
		}
		parts[i] = v
	}
	v := parts[0]
	if len(fields) > 1 {
		v += parts[1] / 60
	}
	if len(fields) > 2 {
		v += parts[2] / 3600
	}
	if negative {
		v = -v
	}
	if hours {
		v *= 15
	}
	return v, true
}

// SeparationDeg is the great-circle separation in degrees (haversine).
func SeparationDeg(ra1, dec1, ra2, dec2 float64) float64 {
	ra1, dec1, ra2, dec2 = ra1*deg2rad, dec1*deg2rad, ra2*deg2rad, dec2*deg2rad
	s1 := math.Sin((dec2 - dec1) / 2)
	s2 := math.Sin((ra2 - ra1) / 2)
	hv := s1*s1 + math.Cos(dec1)*math.Cos(dec2)*s2*s2
	return 2 * math.Asin(clamp(math.Sqrt(hv), 0, 1)) / deg2rad
}
