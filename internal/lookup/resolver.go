package lookup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/peterbuitho/AstroGoPNG/internal/wcs"
)

const userAgent = "AstroGoPNG (+https://github.com/peterbuitho/AstroGoPNG)"

// Resolver resolves names online (CDS Sesame / SIMBAD TAP), caching every
// answer for the lifetime of the run. Safe for concurrent use: several batch
// workers share one Resolver, and duplicate in-flight lookups are collapsed.
type Resolver struct {
	client  *http.Client
	group   singleflight.Group
	mu      sync.Mutex
	cache   map[string]*ObjectInfo // nil value = looked up, not found
	nearby  map[string]*ObjectInfo
	warning string // set (and lookups disabled) after the first network failure
}

// NewResolver creates a resolver with online lookups enabled.
func NewResolver() *Resolver {
	return &Resolver{
		client: &http.Client{Timeout: 12 * time.Second},
		cache:  map[string]*ObjectInfo{},
		nearby: map[string]*ObjectInfo{},
	}
}

// Warning returns the run-level warning if a network failure occurred.
func (r *Resolver) Warning() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.warning == "" {
		return ""
	}
	return fmt.Sprintf("Online object lookup unavailable (%s); file names were stamped instead.", r.warning)
}

func (r *Resolver) enabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.warning == ""
}

func (r *Resolver) fail(err error) {
	r.mu.Lock()
	if r.warning == "" {
		r.warning = err.Error()
	}
	r.mu.Unlock()
}

func (r *Resolver) httpGet(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// Resolve looks a name up. nil when disabled, not found, or after a failure.
func (r *Resolver) Resolve(ctx context.Context, query string) *ObjectInfo {
	if !r.enabled() {
		return nil
	}
	key := normalize(query)
	if key == "" {
		return nil
	}
	r.mu.Lock()
	if v, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return v
	}
	r.mu.Unlock()

	v, _, _ := r.group.Do("resolve:"+key, func() (any, error) {
		q := query
		if t, ok := caldwellTarget(query); ok {
			q = t
		}
		info, err := r.fetchSesame(ctx, q)
		if err == nil && info == nil {
			if alt, ok := spelledOut(q); ok {
				info, err = r.fetchSesame(ctx, alt)
			}
		}
		if err != nil {
			r.fail(err)
			return (*ObjectInfo)(nil), nil
		}
		r.mu.Lock()
		r.cache[key] = info
		r.mu.Unlock()
		return info, nil
	})
	oi, _ := v.(*ObjectInfo)
	return oi
}

func (r *Resolver) fetchSesame(ctx context.Context, query string) (*ObjectInfo, error) {
	u := sesameURL + url.QueryEscape(strings.TrimSpace(query))
	body, err := r.httpGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("Sesame request failed: %w", err)
	}
	info, err := ParseSesame(string(body))
	if err != nil {
		return nil, err
	}
	return info, nil
}

type coneKind int

const (
	anyDSO coneKind = iota
	namedNebula
)

func (r *Resolver) nearbyDSO(ctx context.Context, ra, dec, radius float64) *ObjectInfo {
	return r.cone(ctx, ra, dec, radius, anyDSO)
}
func (r *Resolver) nearbyNamedNebula(ctx context.Context, ra, dec, radius float64) *ObjectInfo {
	return r.cone(ctx, ra, dec, radius, namedNebula)
}

func (r *Resolver) cone(ctx context.Context, ra, dec, radius float64, kind coneKind) *ObjectInfo {
	if !r.enabled() {
		return nil
	}
	key := fmt.Sprintf("%d|%.2f|%.2f|%.2f", kind, ra, dec, radius)
	r.mu.Lock()
	if v, ok := r.nearby[key]; ok {
		r.mu.Unlock()
		return v
	}
	r.mu.Unlock()

	v, _, _ := r.group.Do("cone:"+key, func() (any, error) {
		info, err := r.coneSearch(ctx, ra, dec, radius, kind)
		if err != nil {
			r.fail(err)
			return (*ObjectInfo)(nil), nil
		}
		r.mu.Lock()
		r.nearby[key] = info
		r.mu.Unlock()
		return info, nil
	})
	oi, _ := v.(*ObjectInfo)
	return oi
}

func (r *Resolver) coneSearch(ctx context.Context, ra, dec, radius float64, kind coneKind) (*ObjectInfo, error) {
	types := dsoTypes
	if kind == namedNebula {
		types = nebulaTypes
	}
	quoted := make([]string, len(types))
	for i, t := range types {
		quoted[i] = "'" + t + "'"
	}
	nameFilter := ""
	if kind == namedNebula {
		nameFilter = " AND i.ids LIKE '%NAME %'"
	}
	adql := fmt.Sprintf(
		"SELECT TOP 400 b.main_id, b.otype, b.ra, b.dec, "+
			"DISTANCE(POINT('ICRS', b.ra, b.dec), POINT('ICRS', %.6f, %.6f)) AS d, i.ids, b.morph_type "+
			"FROM basic AS b JOIN ids AS i ON i.oidref = b.oid "+
			"WHERE CONTAINS(POINT('ICRS', b.ra, b.dec), CIRCLE('ICRS', %.6f, %.6f, %.4f)) = 1 "+
			"AND b.otype IN (%s)%s ORDER BY d ASC",
		ra, dec, ra, dec, radius, strings.Join(quoted, ","), nameFilter,
	)
	u := tapURL + "?request=doQuery&lang=adql&format=tsv&query=" + url.QueryEscape(adql)
	body, err := r.httpGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("SIMBAD request failed: %w", err)
	}
	return PickFromTAPTSV(string(body), kind == namedNebula), nil
}

func (o *ObjectInfo) clone() *ObjectInfo {
	c := *o
	c.Designations = append([]string(nil), o.Designations...)
	c.aliasesNorm = make(map[string]struct{}, len(o.aliasesNorm))
	for k := range o.aliasesNorm {
		c.aliasesNorm[k] = struct{}{}
	}
	return &c
}

// adoptCompanionNebula: if info is a cluster/nebula without a common name,
// borrow the name of the named nebula it sits in. Mutates info (call on a clone).
func (r *Resolver) adoptCompanionNebula(ctx context.Context, info *ObjectInfo) {
	if info.CommonName != "" {
		return
	}
	ot := strings.TrimRight(info.OType, "?")
	host := false
	for _, t := range companionHostTypes {
		if ot == t {
			host = true
		}
	}
	if !host || !info.HasPos {
		return
	}
	neb := r.nearbyNamedNebula(ctx, info.RADeg, info.DecDeg, companionRadiusDeg)
	if neb == nil || neb.sameObject(info) {
		return
	}
	info.CommonName = neb.CommonName
	for _, d := range neb.Designations {
		found := false
		for _, e := range info.Designations {
			if e == d {
				found = true
			}
		}
		if !found {
			info.Designations = append(info.Designations, d)
		}
	}
	for k := range neb.aliasesNorm {
		info.aliasesNorm[k] = struct{}{}
	}
	info.OType = neb.OType
}

func spelledOut(query string) (string, bool) {
	q := collapseWS(query)
	sp := strings.IndexByte(q, ' ')
	if sp < 0 {
		return "", false
	}
	prefix, rest := strings.ToUpper(q[:sp]), q[sp+1:]
	var long string
	switch prefix {
	case "CR":
		long = "Collinder"
	case "MEL":
		long = "Melotte"
	case "CED":
		long = "Cederblad"
	case "B":
		long = "Barnard"
	default:
		return "", false
	}
	return long + " " + rest, true
}

// --- identify -------------------------------------------------------

// Identification is the finished identification for one file.
type Identification struct {
	Label Label
	Note  string
}

type named struct {
	info      *ObjectInfo
	preferred string
	note      string
}

// Identify works out what to stamp, given the header OBJECT (if any), the
// header coordinates (if any) and the file-name stem.
func Identify(ctx context.Context, r *Resolver, headerObject string, coords wcs.SkyCoords, hasCoords bool, stem string) Identification {
	fallback := Identification{Label: Label{Title: stem}}
	if r == nil || !r.enabled() {
		return fallback
	}

	fileDesig, hasFileDesig := DesignationInName(stem)
	header := strings.TrimSpace(strings.Trim(strings.TrimSpace(headerObject), "'"))

	nm := identifyByName(ctx, r, header, fileDesig, hasFileDesig)
	if nm != nil {
		nm.info = nm.info.clone()
		r.adoptCompanionNebula(ctx, nm.info)
	}

	if hasCoords {
		c := coords
		if nm != nil {
			if !nm.info.HasPos {
				return Identification{Label: compose(nm.info, nm.preferred), Note: nm.note}
			}
			sep := c.SeparationTo(nm.info.RADeg, nm.info.DecDeg)
			if sep <= c.ToleranceDeg() {
				if sep <= c.SearchRadiusDeg() {
					if centre := r.nearbyDSO(ctx, c.RADeg, c.DecDeg, c.SearchRadiusDeg()); centre != nil {
						cc := centre.clone()
						r.adoptCompanionNebula(ctx, cc)
						if !cc.sameObject(nm.info) && cc.isNotable() && !sameRegion(cc, nm.info) {
							what := firstNonEmpty(nm.preferred, nm.info.MainID)
							note := fmt.Sprintf("frame is centred on %s; %s is %.1f° off-centre, both in the field", actualTitle(cc), what, sep)
							if nm.note != "" {
								note = nm.note + "; " + note
							}
							return Identification{Label: composePair(nm.info, nm.preferred, cc, c.RADeg, c.DecDeg), Note: note}
						}
					}
				}
				return Identification{Label: compose(nm.info, nm.preferred), Note: nm.note}
			}

			whereFrom := "header coordinates"
			if c.Solved {
				whereFrom = "plate solution"
			}
			what := firstNonEmpty(nm.preferred, nm.info.MainID)
			if actual := r.nearbyDSO(ctx, c.RADeg, c.DecDeg, c.SearchRadiusDeg()); actual != nil {
				ac := actual.clone()
				r.adoptCompanionNebula(ctx, ac)
				if !ac.sameObject(nm.info) && ac.isNotable() {
					return Identification{
						Label: compose(ac, ""),
						Note:  fmt.Sprintf("%s is %.1f° from the %s; the frame is centred on %s, used that", what, sep, whereFrom, actualTitle(ac)),
					}
				}
			}
			note := fmt.Sprintf("%s is %.1f° from the %s (tolerance %.1f°)", what, sep, whereFrom, c.ToleranceDeg())
			if nm.note != "" {
				note = nm.note + "; " + note
			}
			return Identification{Label: compose(nm.info, nm.preferred), Note: note}
		}

		if actual := r.nearbyDSO(ctx, c.RADeg, c.DecDeg, c.SearchRadiusDeg()); actual != nil {
			ac := actual.clone()
			r.adoptCompanionNebula(ctx, ac)
			whereFrom := "header coordinates"
			if c.Solved {
				whereFrom = "plate solution"
			}
			return Identification{Label: compose(ac, ""), Note: "identified from the " + whereFrom}
		}
	} else if nm != nil {
		return Identification{Label: compose(nm.info, nm.preferred), Note: nm.note}
	}

	id := fallback
	if r.enabled() && (header != "" || hasFileDesig) {
		id.Note = "object not found in SIMBAD; used file name"
	}
	return id
}

func identifyByName(ctx context.Context, r *Resolver, header, fileDesig string, hasFileDesig bool) *named {
	if header != "" {
		if info := r.Resolve(ctx, header); info != nil {
			if hasFileDesig {
				if !info.matches(fileDesig) {
					if info2 := r.Resolve(ctx, fileDesig); info2 != nil {
						resolvedAs := ""
						if !normEq(header, info.MainID) {
							resolvedAs = fmt.Sprintf(" (%s)", info.MainID)
						}
						return &named{
							info:      info2,
							preferred: fileDesig,
							note:      fmt.Sprintf("header OBJECT is '%s'%s but file name says %s; used file name", header, resolvedAs, fileDesig),
						}
					}
					pref, _ := DesignationInName(header)
					return &named{
						info:      info,
						preferred: pref,
						note:      fmt.Sprintf("header OBJECT '%s' (%s) does not match file name designation %s", header, info.MainID, fileDesig),
					}
				}
				return &named{info: info, preferred: fileDesig}
			}
			pref, _ := DesignationInName(header)
			return &named{info: info, preferred: pref}
		}
	}
	if hasFileDesig {
		if info := r.Resolve(ctx, fileDesig); info != nil {
			return &named{info: info, preferred: fileDesig}
		}
	}
	return nil
}

func actualTitle(info *ObjectInfo) string { return compose(info, "").Title }

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
