// Package batch finds the files to convert and processes them concurrently:
// a pool of workers (one per CPU by default) each runs the decode → stretch →
// resize → stamp → encode pipeline, and progress is streamed back in
// completion order.
//
// Port of the Rust batch.rs, with Go's worker-pool concurrency.
package batch

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/peterbuitho/AstroGoPNG/internal/fits"
	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
	"github.com/peterbuitho/AstroGoPNG/internal/lookup"
	"github.com/peterbuitho/AstroGoPNG/internal/pixels"
	"github.com/peterbuitho/AstroGoPNG/internal/stamp"
	"github.com/peterbuitho/AstroGoPNG/internal/wcs"
	"github.com/peterbuitho/AstroGoPNG/internal/xisf"
)

var imageExts = []string{".xisf", ".fits", ".fit", ".fts"}
var fitsExts = []string{".fits", ".fit", ".fts"}

// Options configures a run.
type Options struct {
	InputDir  string
	OutputDir string // "" = next to sources
	Recursive bool
	Overwrite bool
	Resize4K  bool
	PNGOnly   bool
	Lookup    bool // resolve the object online; false = plain file name
	FontPath  string
	Files     []string // explicit files; when non-empty, InputDir is not scanned
	// Concurrency is the number of files converted in parallel.
	// Zero means min(runtime.NumCPU(), 8).
	Concurrency int
}

func (o Options) resize4kEffective() bool { return o.Resize4K || o.PNGOnly }

// InputKind is a human-readable description of the input file kind.
func (o Options) InputKind() string {
	if o.PNGOnly {
		return ".png"
	}
	return ".xisf / .fits"
}

// Status of one file.
type Status int

const (
	StatusOK Status = iota
	StatusSkipped
	StatusFailed
)

// Progress is reported once per file, from a single goroutine.
type Progress struct {
	Index, Total int
	Rel          string
	Status       Status
	Err          string
	Label        string // stamped title, when it differs from the file name
	Note         string
}

// Summary is the run result.
type Summary struct {
	Total     int
	Converted int
	Skipped   int
	Failed    int
	Cancelled bool
	Warnings  []string
}

type job struct {
	index    int
	src, dst string
	rel      string
}

type result struct {
	index   int
	rel     string
	status  Status
	err     string
	label   string
	note    string
	written bool
}

// Run executes the whole batch. report is called once per file, in completion
// order, from a single goroutine (so it need not be thread-safe).
func Run(ctx context.Context, opts Options, report func(Progress)) (Summary, error) {
	var summary Summary
	explicit := len(opts.Files) > 0

	if !explicit {
		fi, err := os.Stat(opts.InputDir)
		if err != nil || !fi.IsDir() {
			return summary, fmt.Errorf("input directory not found: %s", opts.InputDir)
		}
	}

	var stamper *stamp.Stamper
	if opts.resize4kEffective() {
		var err error
		if opts.FontPath != "" {
			data, e := os.ReadFile(opts.FontPath)
			if e != nil {
				return summary, fmt.Errorf("cannot read font file %s: %w", opts.FontPath, e)
			}
			if stamper, err = stamp.New(data); err != nil {
				return summary, fmt.Errorf("%s is not a valid TrueType/OpenType font", opts.FontPath)
			}
		} else if stamper, err = stamp.Bundled(); err != nil {
			return summary, err
		}
	}

	var resolver *lookup.Resolver
	if opts.Lookup && stamper != nil {
		resolver = lookup.NewResolver()
	}

	// --- gather files ---
	var files []string
	if explicit {
		files = append(files, opts.Files...)
	} else {
		files = collectFiles(opts.InputDir, opts.Recursive, opts.inputExts())
		sort.Slice(files, func(i, j int) bool {
			return strings.ToLower(files[i]) < strings.ToLower(files[j])
		})
	}
	summary.Total = len(files)
	if len(files) == 0 {
		return summary, nil
	}

	// --- concurrency ---
	workers := opts.Concurrency
	if workers <= 0 {
		if workers = runtime.NumCPU(); workers > 8 {
			workers = 8
		}
	}
	if workers > len(files) {
		workers = len(files)
	}

	jobs := make(chan job)
	results := make(chan result)

	go func() {
		defer close(jobs)
		for i, f := range files {
			rel := relFor(f, opts, explicit)
			select {
			case jobs <- job{index: i, src: f, dst: destPath(f, rel, opts, explicit), rel: rel}:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					return
				}
				results <- process(ctx, j, opts, stamper, resolver)
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	for r := range results {
		switch {
		case r.status == StatusFailed:
			summary.Failed++
		case r.written:
			summary.Converted++
		default:
			summary.Skipped++
		}
		report(Progress{
			Index: r.index + 1, Total: len(files), Rel: r.rel,
			Status: r.status, Err: r.err, Label: r.label, Note: r.note,
		})
	}
	if ctx.Err() != nil {
		summary.Cancelled = true
	}
	if resolver != nil {
		if w := resolver.Warning(); w != "" {
			summary.Warnings = append(summary.Warnings, w)
		}
	}
	return summary, nil
}

func process(ctx context.Context, j job, opts Options, stamper *stamp.Stamper, resolver *lookup.Resolver) result {
	r := result{index: j.index, rel: j.rel}
	isPNG := hasExt(j.src, ".png")
	inPlace := isPNG && sameFile(j.src, j.dst)

	if !inPlace {
		if _, err := os.Stat(j.dst); err == nil && !opts.Overwrite {
			return r // skipped
		}
	}

	data, err := os.ReadFile(j.src)
	if err != nil {
		return fail(r, fmt.Sprintf("cannot read input: %v", err))
	}

	var decoded image.Image
	var headerObject string
	var coords wcs.SkyCoords
	var hasCoords bool

	if isPNG {
		m, e := imageio.Decode(data)
		if e != nil {
			return fail(r, fmt.Sprintf("cannot read PNG: %v", e))
		}
		decoded = m
	} else {
		var raw imageio.Raw
		if hasExtAny(j.src, fitsExts) {
			raw, err = fits.Parse(data)
		} else {
			raw, err = xisf.Parse(data)
		}
		if err != nil {
			return fail(r, err.Error())
		}
		headerObject, coords, hasCoords = raw.Object, raw.Coords, raw.HasCoords
		m, e := pixels.ToImage(raw)
		if e != nil {
			return fail(r, e.Error())
		}
		decoded = m
	}

	if stamper != nil {
		stem := strings.TrimSuffix(filepath.Base(j.dst), filepath.Ext(j.dst))
		label := stamp.Label{Title: stem}
		if resolver != nil {
			id := lookup.Identify(ctx, resolver, headerObject, coords, hasCoords, stem)
			label = stamp.Label{Title: id.Label.Title, Subtitle: id.Label.Subtitle}
			r.note = id.Note
			if id.Label.Title != stem {
				r.label = id.Label.Title
			}
		}
		out, e := stamper.ResizeAndLabel(decoded, label)
		if e != nil {
			return fail(r, e.Error())
		}
		decoded = out
	}

	encoded, e := imageio.Encode(decoded)
	if e != nil {
		return fail(r, fmt.Sprintf("PNG encoding failed: %v", e))
	}
	if dir := filepath.Dir(j.dst); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fail(r, fmt.Sprintf("cannot create output folder: %v", err))
		}
	}
	if err := os.WriteFile(j.dst, encoded, 0o644); err != nil {
		return fail(r, fmt.Sprintf("cannot create %s: %v", j.dst, err))
	}
	r.written = true
	return r
}

func fail(r result, msg string) result {
	r.status = StatusFailed
	r.err = msg
	return r
}

func collectFiles(dir string, recursive bool, exts []string) []string {
	var out []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if recursive {
				out = append(out, collectFiles(p, recursive, exts)...)
			}
			continue
		}
		if hasExtAny(p, exts) {
			out = append(out, p)
		}
	}
	return out
}

func (o Options) inputExts() []string {
	if o.PNGOnly {
		return []string{".png"}
	}
	return imageExts
}

func relFor(f string, opts Options, explicit bool) string {
	if explicit {
		return filepath.Base(f)
	}
	if r, err := filepath.Rel(opts.InputDir, f); err == nil {
		return r
	}
	return filepath.Base(f)
}

func destPath(src, rel string, opts Options, explicit bool) string {
	swap := func(p string) string { return strings.TrimSuffix(p, filepath.Ext(p)) + ".png" }
	switch {
	case opts.OutputDir != "":
		return swap(filepath.Join(opts.OutputDir, rel))
	case explicit:
		return swap(src)
	default:
		return swap(filepath.Join(opts.InputDir, rel))
	}
}

func hasExt(p, ext string) bool { return strings.EqualFold(filepath.Ext(p), ext) }

func hasExtAny(p string, exts []string) bool {
	e := strings.ToLower(filepath.Ext(p))
	for _, x := range exts {
		if e == x {
			return true
		}
	}
	return false
}

func sameFile(a, b string) bool {
	ap, err1 := filepath.Abs(a)
	bp, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return strings.EqualFold(ap, bp)
}
