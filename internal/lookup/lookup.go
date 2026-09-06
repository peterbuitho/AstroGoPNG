// Package lookup identifies the object in an image and formats its catalogue
// information into the two-line label to stamp. The network Resolver lives in
// resolver.go; this file is the pure identity + formatting logic and the
// SIMBAD response parsers.
//
// Port of the Rust lookup.rs.
package lookup

import (
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	sesameURL = "https://cds.unistra.fr/cgi-bin/nph-sesame/-oxI/S?"
	tapURL    = "https://simbad.cds.unistra.fr/simbad/sim-tap/sync"

	companionRadiusDeg = 0.5
)

var dsoTypes = []string{
	"G", "AGN", "GiG", "GiP", "GiC", "IG", "PaG", "GrG", "ClG", "SBG", "EmG", "LIN", "SyG", "Sy1",
	"Sy2", "HII", "PN", "SNR", "RNe", "DNe", "GNe", "MoC", "Cld", "ISM", "EmO", "bub", "OpC",
	"GlC", "Cl*", "As*", "SFR", "glb",
}
var nebulaTypes = []string{
	"HII", "RNe", "DNe", "GNe", "MoC", "Cld", "ISM", "EmO", "bub", "SNR", "SFR", "PN",
}
var companionHostTypes = []string{
	"OpC", "Cl*", "As*", "SFR", "HII", "GNe", "ISM", "Cld", "MoC", "EmO", "RNe", "DNe",
}

// Label is what gets stamped (mirrors stamp.Label, kept separate to avoid an
// import cycle).
type Label struct {
	Title    string
	Subtitle string
}

type catalog struct {
	keys         []string
	pretty       string
	simbad       []string
	max          int
	adjacentOnly bool
}

var catalogs = []catalog{
	{[]string{"M", "MESSIER"}, "M ", []string{"M "}, 110, true},
	{[]string{"C", "CALDWELL"}, "C ", nil, 109, true},
	{[]string{"NGC"}, "NGC ", []string{"NGC "}, 7840, false},
	{[]string{"IC"}, "IC ", []string{"IC "}, 5386, false},
	{[]string{"SH2", "SH"}, "Sh2-", []string{"SH 2-", "SH2-"}, 313, false},
	{[]string{"B", "BARNARD"}, "Barnard ", []string{"Barnard "}, 370, true},
	{[]string{"LBN"}, "LBN ", []string{"LBN "}, 1125, false},
	{[]string{"LDN"}, "LDN ", []string{"LDN "}, 1802, false},
	{[]string{"VDB"}, "vdB ", []string{"VdB ", "vdB "}, 158, false},
	{[]string{"CR", "COLLINDER"}, "Cr ", []string{"Cr ", "Cl Collinder "}, 471, false},
	{[]string{"MEL", "MELOTTE"}, "Mel ", []string{"Cl Melotte ", "Mel "}, 245, false},
	{[]string{"CED", "CEDERBLAD"}, "Ced ", []string{"Ced "}, 215, false},
	{[]string{"ARP"}, "Arp ", []string{"APG ", "Arp "}, 338, false},
	{[]string{"UGC"}, "UGC ", []string{"UGC "}, 12921, false},
	{[]string{"PGC"}, "PGC ", []string{"LEDA ", "PGC "}, 9999999, false},
	{[]string{"HD"}, "HD ", []string{"HD "}, 359083, false},
	{[]string{"HIP"}, "HIP ", []string{"HIP "}, 120404, false},
}

// --- normalisation --------------------------------------------------------

var normReplacements = [][2]string{
	{"MESSIER", "M"}, {"CALDWELL", "C"}, {"CLMELOTTE", "MEL"}, {"MELOTTE", "MEL"},
	{"CLCOLLINDER", "CR"}, {"COLLINDER", "CR"}, {"CEDERBLAD", "CED"}, {"LEDA", "PGC"},
	{"APG", "ARP"}, {"SH2-", "SH2-"},
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		b.WriteRune(upper(r))
	}
	out := b.String()
	for _, p := range normReplacements {
		if strings.HasPrefix(out, p[0]) {
			out = p[1] + out[len(p[0]):]
			break
		}
	}
	return out
}

func normEq(a, b string) bool { return normalize(a) == normalize(b) }

func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

func upper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}

// --- ObjectInfo ---------------------------------------------------------

// ObjectInfo is what Sesame / SIMBAD told us about one object.
type ObjectInfo struct {
	MainID       string
	CommonName   string // "" if none
	Designations []string
	aliasesNorm  map[string]struct{}
	OType        string
	MorphType    string // "" if none
	RADeg        float64
	DecDeg       float64
	HasPos       bool
}

func (o *ObjectInfo) matches(designation string) bool {
	if _, ok := o.aliasesNorm[normalize(designation)]; ok {
		return true
	}
	if n, ok := caldwellNumber(o.Designations); ok {
		return normEq(designation, fmt.Sprintf("C %d", n))
	}
	return false
}

func (o *ObjectInfo) sameObject(other *ObjectInfo) bool {
	for k := range o.aliasesNorm {
		if _, ok := other.aliasesNorm[k]; ok {
			return true
		}
	}
	return false
}

func (o *ObjectInfo) isNotable() bool { return o.prominence() <= 5 || o.CommonName != "" }

func (o *ObjectInfo) prominence() int {
	for tier, c := range catalogs {
		for _, d := range o.Designations {
			if strings.HasPrefix(d, c.pretty) {
				return tier
			}
		}
	}
	if o.CommonName != "" {
		return len(catalogs)
	}
	return math.MaxInt
}

func (o *ObjectInfo) typeDescription() string {
	if isGalaxyType(o.OType) && o.MorphType != "" {
		if m := morphologyDescription(o.MorphType); m != "" {
			return m
		}
	}
	return otypeDescription(o.OType)
}

func (o *ObjectInfo) coordinates() string {
	if !o.HasPos {
		return ""
	}
	return fmt.Sprintf("RA %s  Dec %s", fmtRA(o.RADeg), fmtDec(o.DecDeg))
}

func objectFromAliases(mainID string, aliases []string, otype, morph string, ra, dec float64, hasPos bool) *ObjectInfo {
	mainID = collapseWS(mainID)
	mainID = strings.TrimPrefix(mainID, "NAME ")

	var designations []string
	seen := map[string]struct{}{}
	for _, c := range catalogs {
		for _, a := range aliases {
			if d, ok := catalogDesignation(c, a); ok {
				if _, dup := seen[d]; !dup {
					seen[d] = struct{}{}
					designations = append(designations, d)
				}
			}
		}
	}

	aliasesNorm := map[string]struct{}{}
	for _, a := range aliases {
		aliasesNorm[normalize(a)] = struct{}{}
	}

	if n, ok := caldwellNumber(designations); ok {
		c := fmt.Sprintf("C %d", n)
		if _, dup := seen[c]; !dup {
			pos := 0
			for pos < len(designations) && strings.HasPrefix(designations[pos], "M ") {
				pos++
			}
			designations = append(designations, "")
			copy(designations[pos+1:], designations[pos:])
			designations[pos] = c
		}
	}

	common, _ := popularName(designations)
	if common == "" {
		common = pickCommonName(aliases)
	}

	return &ObjectInfo{
		MainID: mainID, CommonName: common, Designations: designations,
		aliasesNorm: aliasesNorm, OType: strings.TrimSpace(otype), MorphType: cleanMorph(morph),
		RADeg: ra, DecDeg: dec, HasPos: hasPos,
	}
}

func catalogDesignation(c catalog, alias string) (string, bool) {
	for _, prefix := range c.simbad {
		if len(alias) > len(prefix) && strings.EqualFold(alias[:len(prefix)], prefix) {
			rest := strings.TrimSpace(alias[len(prefix):])
			if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= c.max {
				return fmt.Sprintf("%s%d", c.pretty, n), true
			}
		}
	}
	return "", false
}

func cleanMorph(m string) string {
	m = strings.TrimSpace(m)
	if m == "" || m == "~" {
		return ""
	}
	return m
}

// --- type descriptions -----------------------------------------------

func isGalaxyType(otype string) bool {
	otype = strings.TrimRight(otype, "?")
	for _, g := range []string{
		"G", "AGN", "GiG", "GiP", "GiC", "BiC", "SBG", "EmG", "H2G", "LSB", "rG",
		"SyG", "Sy1", "Sy2", "LIN", "IG", "PaG", "BLL", "Bla", "QSO",
	} {
		if otype == g {
			return true
		}
	}
	return false
}

func morphologyDescription(code string) string {
	var b strings.Builder
	for _, r := range code {
		if r != ' ' && r != '\t' {
			b.WriteRune(r)
		}
	}
	c := b.String()
	if c == "" {
		return ""
	}
	dwarf := false
	body := c
	if len(c) >= 2 && c[0] == 'd' && c[1] >= 'A' && c[1] <= 'Z' {
		dwarf = true
		body = c[1:]
	}
	up := strings.ToUpper(body)

	var class string
	switch {
	case strings.HasPrefix(up, "SPH") || strings.HasPrefix(up, "DSPH"):
		class = "Spheroidal"
	case strings.HasPrefix(up, "CD"):
		class = "Giant elliptical"
	case strings.HasPrefix(up, "E"):
		class = "Elliptical"
	case strings.HasPrefix(up, "S0") || strings.HasPrefix(up, "SA0") || strings.HasPrefix(up, "SB0") || strings.HasPrefix(up, "SAB0"):
		class = "Lenticular"
	case strings.HasPrefix(up, "SB"):
		class = "Barred spiral"
	case strings.HasPrefix(up, "SA") || strings.HasPrefix(up, "S"):
		class = "Spiral"
	case strings.HasPrefix(up, "I"):
		class = "Irregular"
	case strings.HasPrefix(up, "RING"):
		class = "Ring"
	default:
		return ""
	}
	if dwarf {
		switch class {
		case "Elliptical":
			return "Dwarf elliptical galaxy"
		case "Spheroidal":
			return "Dwarf spheroidal galaxy"
		case "Irregular":
			return "Dwarf irregular galaxy"
		case "Spiral", "Barred spiral":
			return "Dwarf spiral galaxy"
		default:
			return "Dwarf galaxy"
		}
	}
	switch class {
	case "Spheroidal":
		return "Spheroidal galaxy"
	case "Giant elliptical":
		return "Giant elliptical galaxy"
	case "Elliptical":
		return "Elliptical galaxy"
	case "Lenticular":
		return "Lenticular galaxy"
	case "Barred spiral":
		return "Barred spiral galaxy"
	case "Spiral":
		return "Spiral galaxy"
	case "Irregular":
		return "Irregular galaxy"
	case "Ring":
		return "Ring galaxy"
	}
	return ""
}

var otypeTable = map[string]string{
	"G": "Galaxy", "AGN": "Galaxy (active nucleus)", "GiG": "Galaxy in a group",
	"GiP": "Galaxy in a pair", "GiC": "Galaxy in a cluster", "BiC": "Brightest cluster galaxy",
	"IG": "Interacting galaxies", "PaG": "Pair of galaxies", "GrG": "Group of galaxies",
	"CGG": "Compact group of galaxies", "ClG": "Cluster of galaxies", "SCG": "Supercluster of galaxies",
	"SBG": "Starburst galaxy", "EmG": "Emission-line galaxy", "H2G": "HII galaxy",
	"LSB": "Low surface brightness galaxy", "rG": "Radio galaxy", "SyG": "Seyfert galaxy",
	"Sy1": "Seyfert 1 galaxy", "Sy2": "Seyfert 2 galaxy", "LIN": "LINER galaxy", "QSO": "Quasar",
	"BLL": "Blazar", "Bla": "Blazar", "PoG": "Part of a galaxy",
	"HII": "HII region (emission nebula)", "PN": "Planetary nebula", "SNR": "Supernova remnant",
	"RNe": "Reflection nebula", "DNe": "Dark nebula", "GNe": "Nebula", "Neb": "Nebula",
	"EmO": "Emission object", "MoC": "Molecular cloud", "Cld": "Cloud", "ISM": "Interstellar medium",
	"bub": "Bubble", "HH": "Herbig-Haro object", "SFR": "Star-forming region", "PoC": "Part of a cloud",
	"glb": "Globule", "cor": "Dense core", "out": "Outflow", "sh": "Interstellar shell", "reg": "Region",
	"OpC": "Open cluster", "GlC": "Globular cluster", "Cl*": "Star cluster", "As*": "Stellar association",
	"MGr": "Moving group", "St*": "Stellar stream", "*": "Star", "**": "Double or multiple star",
	"V*": "Variable star", "Pe*": "Peculiar star", "Em*": "Emission-line star", "Be*": "Be star",
	"WR*": "Wolf-Rayet star", "Ce*": "Cepheid variable", "Mi*": "Mira variable", "LP*": "Long-period variable",
	"RR*": "RR Lyrae variable", "EB*": "Eclipsing binary", "SB*": "Spectroscopic binary", "Or*": "Orion variable",
	"TT*": "T Tauri star", "Y*O": "Young stellar object", "sg*": "Supergiant", "s*b": "Blue supergiant",
	"s*r": "Red supergiant", "s*y": "Yellow supergiant", "RG*": "Red giant", "AB*": "AGB star",
	"pA*": "Post-AGB star", "C*": "Carbon star", "WD*": "White dwarf", "N*": "Neutron star",
	"Psr": "Pulsar", "BH": "Black hole", "XB*": "X-ray binary", "SN*": "Supernova", "No*": "Nova",
	"Sy*": "Symbiotic star", "PM*": "High proper-motion star", "HS*": "Hot subdwarf", "BD*": "Brown dwarf",
	"LM*": "Low-mass star", "Pl": "Exoplanet", "gLe": "Gravitational lens", "X": "X-ray source",
	"Rad": "Radio source", "IR": "Infrared source", "UV": "UV source", "gam": "Gamma-ray source",
}

func otypeDescription(code string) string {
	code = strings.TrimRight(code, "?")
	if v, ok := otypeTable[code]; ok {
		return v
	}
	if code == "err" || code == "?" || code == "" {
		return ""
	}
	return code
}

// --- common-name selection ------------------------------------------

var typeWords = map[string]struct{}{}
var constellationAbbr = map[string]struct{}{}
var otherAbbr = map[string]struct{}{}

func init() {
	for _, w := range []string{
		"nebula", "galaxy", "cluster", "cloud", "remnant", "loop", "complex", "star", "group",
		"association", "region", "filament", "chain", "triplet", "quintet", "sextet", "arc",
		"wall", "bubble", "shell", "ring", "pair", "stream", "dwarf",
	} {
		typeWords[w] = struct{}{}
	}
	for _, w := range []string{
		"And", "Ant", "Aps", "Aqr", "Aql", "Ara", "Ari", "Aur", "Boo", "Cae", "Cam", "Cnc", "CVn",
		"CMa", "CMi", "Cap", "Car", "Cas", "Cen", "Cep", "Cet", "Cha", "Cir", "Col", "Com", "CrA",
		"CrB", "Crv", "Crt", "Cru", "Cyg", "Del", "Dor", "Dra", "Equ", "Eri", "For", "Gem", "Gru",
		"Her", "Hor", "Hya", "Hyi", "Ind", "Lac", "Leo", "LMi", "Lep", "Lib", "Lup", "Lyn", "Lyr",
		"Men", "Mic", "Mon", "Mus", "Nor", "Oct", "Oph", "Ori", "Pav", "Peg", "Per", "Phe", "Pic",
		"Psc", "PsA", "Pup", "Pyx", "Ret", "Sge", "Sgr", "Sco", "Scl", "Sct", "Ser", "Sex", "Tau",
		"Tel", "Tri", "TrA", "Tuc", "UMa", "UMi", "Vel", "Vir", "Vol", "Vul",
	} {
		constellationAbbr[w] = struct{}{}
	}
	for _, w := range []string{"Neb", "Gal", "Cl", "Nebul", "Amer"} {
		otherAbbr[w] = struct{}{}
	}
}

func endsWithTypeWord(name string) bool {
	f := strings.Fields(name)
	if len(f) == 0 {
		return false
	}
	_, ok := typeWords[strings.ToLower(f[len(f)-1])]
	return ok
}

func isCleanName(name string) bool {
	words := 0
	for _, w := range strings.Fields(name) {
		words++
		alnum := 0
		var last rune
		for _, r := range w {
			if r >= '0' && r <= '9' {
				return false
			}
			if isAlnum(r) {
				alnum++
				last = r
			}
		}
		if alnum == 1 && last >= 'A' && last <= 'Z' {
			return false
		}
	}
	return words > 0
}

func isAlnum(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func pickCommonName(aliases []string) string {
	var candidates []string
	for _, a := range aliases {
		if !strings.HasPrefix(a, "NAME ") {
			continue
		}
		nm := strings.TrimSpace(a[len("NAME "):])
		if nm == "" || !isCleanName(nm) {
			continue
		}
		candidates = append(candidates, nm)
	}
	if len(candidates) == 0 {
		return ""
	}
	var nice []string
	for _, nm := range candidates {
		if !isShouting(nm) && !hasAbbrev(nm) {
			nice = append(nice, nm)
		}
	}
	for _, nm := range nice {
		if endsWithTypeWord(nm) {
			return nm
		}
	}
	if len(nice) > 0 {
		return nice[0]
	}
	return candidates[0]
}

func isShouting(n string) bool {
	letters, allUpper := 0, true
	for _, r := range n {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			letters++
			if r >= 'a' && r <= 'z' {
				allUpper = false
			}
		}
	}
	return letters >= 4 && allUpper
}

func hasAbbrev(n string) bool {
	for _, w := range strings.Fields(n) {
		w = strings.Trim(w, ".,:;!?'\"()")
		if _, ok := constellationAbbr[w]; ok {
			return true
		}
		if _, ok := otherAbbr[w]; ok {
			return true
		}
	}
	return false
}

// --- file-name designation scanner --------------------------------

// DesignationInName finds the first catalogue designation in a file-name stem,
// e.g. "2026-09-05_NGC7000_Ha" -> "NGC 7000".
func DesignationInName(name string) (string, bool) {
	upper := strings.ToUpper(name)
	n := len(upper)
	isSep := func(c byte) bool { return c == ' ' || c == '_' || c == '-' || c == '.' }
	isAlpha := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	isAlnumB := func(c byte) bool { return isAlpha(c) || isDigit(c) }

	i := 0
	for i < n {
		if !isAlpha(upper[i]) || (i > 0 && isAlnumB(upper[i-1])) {
			i++
			continue
		}
		start := i
		for i < n && isAlpha(upper[i]) {
			i++
		}
		word := upper[start:i]

		for _, c := range catalogs {
			hit := false
			for _, k := range c.keys {
				if k == word {
					hit = true
				}
			}
			if !hit {
				continue
			}
			j := i
			if word == "SH" {
				if j < n && isSep(upper[j]) {
					j++
				}
				if j < n && upper[j] == '2' {
					j++
				} else {
					continue
				}
			}
			sepHere := j < n && isSep(upper[j])
			if sepHere {
				if c.adjacentOnly {
					continue
				}
				j++
			}
			dstart := j
			for j < n && isDigit(upper[j]) && j-dstart < 8 {
				j++
			}
			if j == dstart || (j < n && isAlpha(upper[j])) {
				continue
			}
			if num, err := strconv.Atoi(upper[dstart:j]); err == nil && num >= 1 && num <= c.max {
				return fmt.Sprintf("%s%d", c.pretty, num), true
			}
		}
	}
	return "", false
}

// --- coordinate formatting -------------------------------------

func fmtRA(deg float64) string {
	hours := mod(deg, 360) / 15
	total := int64(math.Round(hours * 3600))
	h := total / 3600 % 24
	m := total / 60 % 60
	s := total % 60
	return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
}

func fmtDec(deg float64) string {
	sign := "+"
	if deg < 0 {
		sign = "−"
	}
	total := int64(math.Round(math.Abs(deg) * 3600))
	d := total / 3600
	m := total / 60 % 60
	s := total % 60
	return fmt.Sprintf("%s%02d° %02d′ %02d″", sign, d, m, s)
}

func mod(a, b float64) float64 {
	r := math.Mod(a, b)
	if r < 0 {
		r += b
	}
	return r
}

// --- Sesame / TAP parsers ------------------------------------

type sesameResolver struct {
	XMLName xml.Name `xml:"Resolver"`
	OName   string   `xml:"oname"`
	OType   string   `xml:"otype"`
	MType   string   `xml:"MType"`
	JRADeg  string   `xml:"jradeg"`
	JDEDeg  string   `xml:"jdedeg"`
	Aliases []string `xml:"alias"`
}

// ParseSesame parses Sesame's -ox XML output. (nil, nil) = nothing found.
func ParseSesame(src string) (*ObjectInfo, error) {
	dec := xml.NewDecoder(strings.NewReader(src))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Resolver" {
			continue
		}
		var r sesameResolver
		if err := dec.DecodeElement(&r, &se); err != nil {
			return nil, fmt.Errorf("sesame XML invalid: %w", err)
		}
		if strings.TrimSpace(r.OName) == "" {
			continue
		}
		mainID := collapseWS(r.OName)
		mainID = strings.TrimPrefix(mainID, "NAME ")
		var aliases []string
		for _, a := range r.Aliases {
			if s := collapseWS(a); s != "" {
				aliases = append(aliases, s)
			}
		}
		aliases = append(aliases, mainID)
		ra, hasRA := parseF(r.JRADeg)
		de, hasDE := parseF(r.JDEDeg)
		return objectFromAliases(mainID, aliases, strings.TrimSpace(r.OType), r.MType, ra, de, hasRA && hasDE), nil
	}
	return nil, nil
}

func parseF(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil
}

// PickFromTAPTSV picks the best hit from a SIMBAD TAP TSV result.
func PickFromTAPTSV(tsv string, requireName bool) *ObjectInfo {
	var best *ObjectInfo
	bestRank := [2]int{math.MaxInt, math.MaxInt}
	lines := strings.Split(tsv, "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 6 {
			continue
		}
		unq := func(s string) string { return strings.Trim(strings.TrimSpace(s), "\"") }
		morph := ""
		if len(cols) >= 7 {
			morph = unq(cols[6])
		}
		ra, _ := parseF(cols[2])
		de, hasDE := parseF(cols[3])
		_, hasRA := parseF(cols[2])
		info := objectFromTAP(unq(cols[0]), unq(cols[5]), unq(cols[1]), morph, ra, de, hasRA && hasDE)
		tier := info.prominence()
		if tier == math.MaxInt {
			continue
		}
		var rank [2]int
		if requireName {
			if info.CommonName == "" || !isCleanName(info.CommonName) {
				continue
			}
			nt := 1
			if endsWithTypeWord(info.CommonName) {
				nt = 0
			}
			rank = [2]int{nt, tier}
		} else {
			rank = [2]int{0, tier}
		}
		if rankLess(rank, bestRank) {
			bestRank = rank
			best = info
			if rank == [2]int{0, 0} {
				break
			}
		}
	}
	return best
}

func rankLess(a, b [2]int) bool {
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	return a[1] < b[1]
}

func objectFromTAP(mainID, ids, otype, morph string, ra, dec float64, hasPos bool) *ObjectInfo {
	mid := collapseWS(mainID)
	mid = strings.TrimPrefix(mid, "NAME ")
	var aliases []string
	for _, a := range strings.Split(ids, "|") {
		if s := collapseWS(a); s != "" {
			aliases = append(aliases, s)
		}
	}
	aliases = append(aliases, mid)
	return objectFromAliases(mid, aliases, otype, morph, ra, dec, hasPos)
}

// --- label composition -----------------------------------------

func titleDesignation(info *ObjectInfo, preferred string) string {
	if preferred != "" {
		return preferred
	}
	for _, d := range info.Designations {
		if !strings.HasPrefix(d, "C ") {
			return d
		}
	}
	if len(info.Designations) > 0 {
		return info.Designations[0]
	}
	return info.MainID
}

func titleOf(info *ObjectInfo, designation string) string {
	if info.CommonName != "" && !normEq(info.CommonName, designation) {
		return fmt.Sprintf("%s (%s)", info.CommonName, designation)
	}
	return designation
}

func objectParts(info *ObjectInfo, designation string, maxIDs int) []string {
	key := normalize(designation)
	var parts []string
	for _, d := range info.Designations {
		if normalize(d) == key {
			continue
		}
		if len(parts) >= maxIDs {
			break
		}
		parts = append(parts, d)
	}
	if ty := info.typeDescription(); ty != "" {
		parts = append(parts, ty)
	}
	return parts
}

func compose(info *ObjectInfo, preferred string) Label {
	designation := titleDesignation(info, preferred)
	parts := objectParts(info, designation, 3)
	if c := info.coordinates(); c != "" {
		parts = append(parts, c)
	}
	sub := ""
	if len(parts) > 0 {
		sub = strings.Join(parts, "  ·  ")
	}
	return Label{Title: titleOf(info, designation), Subtitle: sub}
}

func composePair(first *ObjectInfo, firstPref string, second *ObjectInfo, centreRA, centreDec float64) Label {
	d1 := titleDesignation(first, firstPref)
	d2 := titleDesignation(second, "")
	var title string
	if first.CommonName != "" && second.CommonName != "" && normEq(first.CommonName, second.CommonName) {
		title = fmt.Sprintf("%s (%s & %s)", first.CommonName, d1, d2)
	} else {
		title = fmt.Sprintf("%s & %s", titleOf(first, d1), titleOf(second, d2))
	}
	var parts []string
	if p := strings.Join(objectParts(first, d1, 2), "  ·  "); p != "" {
		parts = append(parts, p)
	}
	if p := strings.Join(objectParts(second, d2, 2), "  ·  "); p != "" {
		parts = append(parts, p)
	}
	parts = append(parts, fmt.Sprintf("RA %s  Dec %s", fmtRA(centreRA), fmtDec(centreDec)))
	return Label{Title: title, Subtitle: strings.Join(parts, "   +   ")}
}

// sameRegion reports whether two records describe the same region under
// slightly different names.
func sameRegion(a, b *ObjectInfo) bool {
	if a.CommonName == "" || b.CommonName == "" {
		return false
	}
	if normEq(a.CommonName, b.CommonName) {
		return false
	}
	wa := wordSet(a.CommonName)
	wb := wordSet(b.CommonName)
	shared := 0
	for w := range wa {
		if _, ok := wb[w]; ok {
			shared++
		}
	}
	if shared >= 2 {
		return true
	}
	return subset(wa, wb) || subset(wb, wa)
}

func wordSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, w := range strings.Fields(s) {
		w = strings.TrimSuffix(strings.TrimSuffix(w, "'s"), "’s")
		var b strings.Builder
		for _, r := range strings.ToLower(w) {
			if isAlnum(r) {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			out[b.String()] = struct{}{}
		}
	}
	return out
}

func subset(small, big map[string]struct{}) bool {
	if len(small) == 0 {
		return false
	}
	for w := range small {
		if _, ok := big[w]; !ok {
			return false
		}
	}
	return true
}

// sorted keys helper (kept for deterministic behaviour in a couple of spots).
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var _ = sortedKeys
