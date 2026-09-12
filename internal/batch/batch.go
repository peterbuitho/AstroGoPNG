// Package batch is a cgo wrapper around astropng-core's C ABI
// (github.com/peterbuitho/astropng-core), which holds the actual conversion
// pipeline (XISF/FITS parsing, stretch, resize/stamp, WCS, SIMBAD lookup,
// worker-pool batch orchestration). Also used, via the same C ABI, by the
// Nim/Zig/Scala ports of this program.
//
// The exported API here (Options/Progress/Summary/Status/Run) is unchanged
// from before this package called into the shared core, so cmd/astrogopng
// and gui/ need no changes.
package batch

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/astropng-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/astropng-core/lib -lastropng_core -lpthread -ldl -lm
#include <stdlib.h>
#include "astropng_core.h"

extern void goProgressCallback(AstropngProgress *progress, void *user_data);
*/
import "C"

import (
	"context"
	"fmt"
	"runtime/cgo"
	"unsafe"
)

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
	// Zero means the core's default (min(available_parallelism, 8)).
	Concurrency int
}

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

// Run executes the whole batch by calling astropng_run. report is called
// once per file, in completion order, from a single goroutine (so it need
// not be thread-safe) — astropng_run guarantees callback invocations never
// overlap.
func Run(ctx context.Context, opts Options, report func(Progress)) (Summary, error) {
	var summary Summary

	cOpts, freeOpts, err := toCOptions(opts)
	if err != nil {
		return summary, err
	}
	defer freeOpts()

	handle := cgo.NewHandle(report)
	defer handle.Delete()

	cancel := C.astropng_cancel_token_new()
	defer C.astropng_cancel_token_free(cancel)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			C.astropng_cancel_token_cancel(cancel)
		case <-done:
		}
	}()

	var outSummary *C.AstropngSummary
	var outError *C.char
	rc := C.astropng_run(
		cOpts,
		cancel,
		C.AstropngProgressCallback(C.goProgressCallback),
		// cgo.Handle-as-void* is the documented idiom for passing a Go value
		// through a C callback's user_data (see runtime/cgo docs); go vet's
		// unsafeptr check can't tell this from real pointer misuse and flags
		// it as a false positive.
		unsafe.Pointer(uintptr(handle)),
		&outSummary,
		&outError,
	)

	if rc != 0 {
		msg := C.GoString(outError)
		C.astropng_string_free(outError)
		return summary, fmt.Errorf("%s", msg)
	}
	defer C.astropng_summary_free(outSummary)

	summary = Summary{
		Total:     int(outSummary.total),
		Converted: uint32ToInt(outSummary.converted),
		Skipped:   uint32ToInt(outSummary.skipped),
		Failed:    uint32ToInt(outSummary.failed),
		Cancelled: bool(outSummary.cancelled),
		Warnings:  goStrings(outSummary.warnings, outSummary.warnings_len),
	}
	return summary, nil
}

//export goProgressCallback
func goProgressCallback(progress *C.AstropngProgress, userData unsafe.Pointer) {
	handle := cgo.Handle(uintptr(userData))
	report := handle.Value().(func(Progress))

	var status Status
	switch progress.status {
	case C.AstropngFileStatus_Ok:
		status = StatusOK
	case C.AstropngFileStatus_Skipped:
		status = StatusSkipped
	case C.AstropngFileStatus_Failed:
		status = StatusFailed
	}

	report(Progress{
		Index:  int(progress.index),
		Total:  int(progress.total),
		Rel:    C.GoString(progress.rel_path),
		Status: status,
		Err:    goStringOpt(progress.status_message),
		Label:  goStringOpt(progress.label),
		Note:   goStringOpt(progress.note),
	})
}

// toCOptions converts Options to a C.AstropngOptions and returns a function
// that frees every allocation it made. The returned pointer (and everything
// it references) is only valid until that function is called.
func toCOptions(opts Options) (*C.AstropngOptions, func(), error) {
	var toFree []unsafe.Pointer
	free := func() {
		for _, p := range toFree {
			C.free(p)
		}
	}
	cstr := func(s string) *C.char {
		p := C.CString(s)
		toFree = append(toFree, unsafe.Pointer(p))
		return p
	}
	cstrOpt := func(s string) *C.char {
		if s == "" {
			return nil
		}
		return cstr(s)
	}

	c := &C.AstropngOptions{
		input_dir:   cstr(opts.InputDir),
		output_dir:  cstrOpt(opts.OutputDir),
		recursive:   C.bool(opts.Recursive),
		overwrite:   C.bool(opts.Overwrite),
		resize4k:    C.bool(opts.Resize4K),
		png_only:    C.bool(opts.PNGOnly),
		font_path:   cstrOpt(opts.FontPath),
		lookup:      C.bool(opts.Lookup),
		concurrency: C.size_t(opts.Concurrency),
	}

	if len(opts.Files) > 0 {
		files := C.malloc(C.size_t(len(opts.Files)) * C.size_t(unsafe.Sizeof(uintptr(0))))
		toFree = append(toFree, files)
		slice := unsafe.Slice((**C.char)(files), len(opts.Files))
		for i, f := range opts.Files {
			slice[i] = cstr(f)
		}
		c.files = (**C.char)(files)
		c.files_len = C.size_t(len(opts.Files))
	}

	return c, free, nil
}

func goStringOpt(s *C.char) string {
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

func goStrings(arr **C.char, n C.size_t) []string {
	if arr == nil || n == 0 {
		return nil
	}
	slice := unsafe.Slice(arr, int(n))
	out := make([]string, len(slice))
	for i, p := range slice {
		out[i] = C.GoString(p)
	}
	return out
}

func uint32ToInt(v C.uint32_t) int { return int(v) }
