package wcs

import (
	"math"
	"testing"
)

func mapGetter(pairs map[string]string) Getter {
	return func(k string) (string, bool) { v, ok := pairs[k]; return v, ok }
}

func TestAngles(t *testing.T) {
	check := func(s string, hours bool, want float64) {
		t.Helper()
		got, ok := ParseAngle(s, hours)
		if !ok || math.Abs(got-want) > 1e-4 {
			t.Errorf("ParseAngle(%q,%v) = %v,%v want %v", s, hours, got, ok, want)
		}
	}
	check("05 35 17.3", true, 83.822083)
	check("05:35:17", true, 83.820833)
	check("-05 23 28", false, -5.391111)
	check("+41 16 08", false, 41.268889)
	check("83.82", false, 83.82)
	check("5.5", true, 82.5)
	if _, ok := ParseAngle("", false); ok {
		t.Error("empty should fail")
	}
}

func TestWCSCentreAndFOV(t *testing.T) {
	m := mapGetter(map[string]string{
		"CTYPE1": "RA---TAN", "CRVAL1": "314.7", "CRVAL2": "44.33",
		"CRPIX1": "400.5", "CRPIX2": "250.5",
		"CD1_1": "-0.000555556", "CD1_2": "0", "CD2_1": "0", "CD2_2": "0.000555556",
	})
	c, ok := FromKeywords(m, 800, 500)
	if !ok || !c.Solved {
		t.Fatal("expected solved coords")
	}
	if math.Abs(c.RADeg-314.7) > 1e-6 || math.Abs(c.DecDeg-44.33) > 1e-6 {
		t.Errorf("centre = %v,%v", c.RADeg, c.DecDeg)
	}
	if math.Abs(c.FOVRadiusDeg-0.262) > 0.002 {
		t.Errorf("fov = %v", c.FOVRadiusDeg)
	}

	m2 := mapGetter(map[string]string{
		"CRVAL1": "100.0", "CRVAL2": "0.0", "CRPIX1": "0.5", "CRPIX2": "0.5",
		"CDELT1": "-0.001", "CDELT2": "0.001",
	})
	c2, _ := FromKeywords(m2, 1000, 1000)
	if math.Abs(c2.RADeg-99.5) > 1e-6 || math.Abs(c2.DecDeg-0.5) > 1e-6 {
		t.Errorf("c2 = %v,%v", c2.RADeg, c2.DecDeg)
	}
}

func TestTargetCoords(t *testing.T) {
	m := mapGetter(map[string]string{
		"OBJCTRA": "00 42 44", "OBJCTDEC": "+41 16 08",
		"XPIXSZ": "3.76", "FOCALLEN": "400", "RA": "999",
	})
	c, ok := FromKeywords(m, 6248, 4176)
	if !ok || c.Solved {
		t.Fatal("expected unsolved coords")
	}
	if math.Abs(c.RADeg-10.6833) > 1e-3 {
		t.Errorf("ra = %v", c.RADeg)
	}
	if !c.FOVKnown || c.FOVRadiusDeg < 1.9 || c.FOVRadiusDeg > 2.1 {
		t.Errorf("fov = %v", c.FOVRadiusDeg)
	}

	m2 := mapGetter(map[string]string{"RA": "83.82", "DEC": "-5.39"})
	c2, _ := FromKeywords(m2, 100, 100)
	if c2.RADeg != 83.82 || c2.DecDeg != -5.39 || c2.FOVKnown {
		t.Errorf("c2 = %+v", c2)
	}

	if _, ok := FromKeywords(mapGetter(nil), 100, 100); ok {
		t.Error("no keywords should fail")
	}
}

func TestSeparation(t *testing.T) {
	if math.Abs(SeparationDeg(10, 40, 10, 40)) > 1e-9 {
		t.Error("zero separation")
	}
	if math.Abs(SeparationDeg(0, 0, 1, 0)-1) > 1e-9 {
		t.Error("1 degree")
	}
	if math.Abs(SeparationDeg(0, 89, 180, 89)-2) > 1e-6 {
		t.Error("2 degrees over the pole")
	}
}
