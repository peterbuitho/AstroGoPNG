package lookup

import "testing"

func TestNormalize(t *testing.T) {
	cases := [][2]string{
		{"M  31", "M31"}, {"SH 2-155", "SH2-155"}, {"Cl Melotte 22", "MEL22"},
		{"Mel 22", "MEL22"}, {"Messier 31", "M31"},
	}
	for _, c := range cases {
		if got := normalize(c[0]); got != c[1] {
			t.Errorf("normalize(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestDesignationInName(t *testing.T) {
	yes := map[string]string{
		"M31_Andromeda_2026-09-05":        "M 31",
		"2026-09-05_NGC7000_Ha_300s_0001": "NGC 7000",
		"ngc_7000 stack":                  "NGC 7000",
		"Sh2-155_RGB":                     "Sh2-155",
		"SH2_101":                         "Sh2-101",
		"IC1396_Elephant":                 "IC 1396",
		"B33_horsehead":                   "Barnard 33",
		"Messier42":                       "M 42",
		"C7_L_300s":                       "C 7",
		"Caldwell14_RGB":                  "C 14",
	}
	for in, want := range yes {
		got, ok := DesignationInName(in)
		if !ok || got != want {
			t.Errorf("DesignationInName(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"Messier 42", "Light_B_120s_0001", "M_31_L", "NGC7000A", "M999", "flat_2026", "C 7", "C200"} {
		if got, ok := DesignationInName(in); ok {
			t.Errorf("DesignationInName(%q) = %q, want no match", in, got)
		}
	}
}

func TestCoordinateFormatting(t *testing.T) {
	if got := fmtRA(10.68470833); got != "00h 42m 44s" {
		t.Errorf("fmtRA = %q", got)
	}
	if got := fmtDec(41.26875); got != "+41° 16′ 08″" {
		t.Errorf("fmtDec = %q", got)
	}
}

func TestMorphologyWords(t *testing.T) {
	cases := map[string]string{
		"SAB(s)cd": "Spiral galaxy", "Sc": "Spiral galaxy", "SB(r)b": "Barred spiral galaxy",
		"E+0-1 pec": "Elliptical galaxy", "S0 pec": "Lenticular galaxy", "IB(s)m": "Irregular galaxy",
		"dE": "Dwarf elliptical galaxy", "dSph": "Dwarf spheroidal galaxy", "cD": "Giant elliptical galaxy",
	}
	for in, want := range cases {
		if got := morphologyDescription(in); got != want {
			t.Errorf("morphologyDescription(%q) = %q want %q", in, got, want)
		}
	}
	if got := morphologyDescription("~"); got != "" {
		t.Errorf("morphologyDescription(~) = %q", got)
	}
}

func TestCaldwell(t *testing.T) {
	if n, ok := caldwellNumber([]string{"NGC 2403"}); !ok || n != 7 {
		t.Errorf("caldwellNumber NGC2403 = %d,%v", n, ok)
	}
	if n, ok := caldwellNumber([]string{"UGC 454", "NGC  884"}); !ok || n != 14 {
		t.Errorf("caldwellNumber = %d,%v", n, ok)
	}
	if _, ok := caldwellNumber([]string{"NGC 224"}); ok {
		t.Error("NGC 224 is not Caldwell")
	}
	if d, ok := caldwellTarget("C7"); !ok || d != "NGC 2403" {
		t.Errorf("caldwellTarget C7 = %q,%v", d, ok)
	}
	if d, ok := caldwellTarget("Caldwell 5"); !ok || d != "IC 342" {
		t.Errorf("caldwellTarget Caldwell 5 = %q,%v", d, ok)
	}
	if _, ok := caldwellTarget("NGC 7"); ok {
		t.Error("NGC 7 should not resolve as Caldwell")
	}
	if len(caldwell) != 109 {
		t.Errorf("caldwell has %d entries", len(caldwell))
	}
}

func TestPopularNames(t *testing.T) {
	cases := map[string]string{
		"IC 342": "Hidden Galaxy", "IC 1848": "Soul Nebula", "M  42": "Orion Nebula",
		"SH 2-158": "Northern Lagoon Nebula", "M 65": "Leo Triplet", "NGC 6357": "War and Peace Nebula",
		"NGC 2537": "Bear's Paw Galaxy",
	}
	for in, want := range cases {
		if got, ok := popularName([]string{in}); !ok || got != want {
			t.Errorf("popularName(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := popularName([]string{"NGC 2403"}); ok {
		t.Error("NGC 2403 has no popular name")
	}
	if got, _ := popularName([]string{"NGC 2682", "M 67"}); got != "Golden Eye Cluster" {
		t.Errorf("M67 popular name = %q", got)
	}
}

func TestParseSesame(t *testing.T) {
	src := `<?xml version="1.0"?><Sesame><Target option="S"><name>M31</name>
	<Resolver name="Sc=Simbad"><otype>AGN</otype><jradeg>10.68470833</jradeg><jdedeg>41.26875</jdedeg>
	<oname>M  31</oname><alias>M 31</alias><alias>NAME Andromeda</alias><alias>NAME Andromeda Galaxy</alias>
	<alias>NAME And Nebula</alias><alias>NGC 224</alias><alias>UGC 454</alias><alias>LEDA 2557</alias>
	</Resolver></Target></Sesame>`
	info, err := ParseSesame(src)
	if err != nil || info == nil {
		t.Fatalf("ParseSesame: %v %v", info, err)
	}
	if info.MainID != "M 31" {
		t.Errorf("MainID = %q", info.MainID)
	}
	if info.CommonName != "Andromeda Galaxy" {
		t.Errorf("CommonName = %q", info.CommonName)
	}
	want := []string{"M 31", "NGC 224", "UGC 454", "PGC 2557"}
	if len(info.Designations) != len(want) {
		t.Fatalf("Designations = %v", info.Designations)
	}
	for i := range want {
		if info.Designations[i] != want[i] {
			t.Errorf("Designations[%d] = %q want %q", i, info.Designations[i], want[i])
		}
	}
	if !info.matches("m31") || !info.matches("NGC224") || info.matches("NGC 7000") {
		t.Error("matches")
	}

	label := compose(info, "M 31")
	if label.Title != "Andromeda Galaxy (M 31)" {
		t.Errorf("title = %q", label.Title)
	}
	wantSub := "NGC 224  ·  UGC 454  ·  PGC 2557  ·  Galaxy (active nucleus)  ·  RA 00h 42m 44s  Dec +41° 16′ 08″"
	if label.Subtitle != wantSub {
		t.Errorf("subtitle =\n %q\nwant\n %q", label.Subtitle, wantSub)
	}

	none, _ := ParseSesame(`<Sesame><Target><name>ZZZ</name><INFO> *** Nothing found *** </INFO></Target></Sesame>`)
	if none != nil {
		t.Error("expected nil for not found")
	}
}

func TestTAPRanking(t *testing.T) {
	tsv := "main_id\totype\tra\tdec\td\tids\n" +
		"\"[PSC2013] 9\"\t\"PN\"\t10.6835\t41.2690\t0.0009\t\"[PSC2013] 9\"\n" +
		"\"Ford M 31 574\"\t\"PN\"\t10.6873\t41.2678\t0.0022\t\"Ford M 31 574|[B2015] M31 B127-33\"\n" +
		"\"NGC  206\"\t\"Cl*\"\t10.10\t40.73\t0.7\t\"NGC   206|OB 78\"\n" +
		"\"M  31\"\t\"AGN\"\t10.6847\t41.2687\t0.9\t\"NAME Andromeda Galaxy|M  31|NGC   224|UGC   454\"\n"
	best := PickFromTAPTSV(tsv, false)
	if best == nil || best.MainID != "M 31" || best.CommonName != "Andromeda Galaxy" {
		t.Fatalf("PickFromTAPTSV = %+v", best)
	}
	only := "main_id\totype\tra\tdec\td\tids\n\"[PSC2013] 9\"\t\"PN\"\t1\t2\t0.1\t\"[PSC2013] 9\"\n"
	if PickFromTAPTSV(only, false) != nil {
		t.Error("only obscure objects should give nil")
	}

	if !isCleanName("Wizard Nebula") || !isCleanName("Barnard's Loop") {
		t.Error("clean names")
	}
	for _, n := range []string{"AFGL 333 Cloud", "Rosette B", "Lo 2"} {
		if isCleanName(n) {
			t.Errorf("%q should not be clean", n)
		}
	}
}
