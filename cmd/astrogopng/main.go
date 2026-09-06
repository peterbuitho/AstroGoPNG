// Command astrogopng batch-converts XISF and FITS astronomical images to PNG,
// optionally resized to 4K with the object name stamped in the corner.
//
// Files are converted in parallel (one worker per CPU by default).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterbuitho/AstroGoPNG/internal/batch"
)

var version = "dev"

const usage = `astrogopng %s - batch convert XISF and FITS astronomical images to PNG

Usage:
  astrogopng [input_dir] [output_dir] [--recursive|-r] [--overwrite] [--resize4k] [--filename] [-j N]
  astrogopng [input_dir] [output_dir] --png-only [--recursive|-r] [--overwrite] [--filename]
  astrogopng <file>... [output_dir] [--overwrite] [--resize4k] [--filename]

Converts every .xisf, .fits, .fit and .fts file found in input_dir, or exactly
the files given (.png files are only resized/stamped). If input_dir is omitted,
the current folder is used. If output_dir is omitted, PNGs are written next to
their source files.

Options:
  -r, --recursive       recurse into subfolders (output mirrors structure)
      --overwrite       overwrite existing .png files (default: skip)
      --resize4k        scale each PNG to exactly 3840x2160 (aspect kept,
                        centre-cropped) and stamp the object name bottom-right,
                        resolved via CDS Sesame / SIMBAD
      --filename        stamp the plain file name: no header OBJECT, no online
                        lookup (aliases: --no-lookup, --offline)
      --png-only        skip XISF/FITS conversion: take existing .png files and
                        only resize/annotate them (implies --resize4k)
      --font <file>     .ttf/.otf font file for the stamp (default: bundled
                        DejaVu Sans Condensed Bold)
  -j, --concurrency N   number of files to convert in parallel
                        (default: number of CPUs, capped at 8)
  -V, --version         print version
  -h, --help            show this help
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts := batch.Options{Lookup: true}
	var inputDir, outputDir string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--recursive" || a == "-r":
			opts.Recursive = true
		case a == "--overwrite":
			opts.Overwrite = true
		case a == "--resize4k" || a == "-resize4k":
			opts.Resize4K = true
		case a == "--png-only":
			opts.PNGOnly = true
		case a == "--filename" || a == "--no-lookup" || a == "--offline":
			opts.Lookup = false
		case a == "--font":
			i++
			if i >= len(args) || args[i] == "" {
				fmt.Fprintln(os.Stderr, "--font requires a path to a .ttf/.otf file")
				return 2
			}
			opts.FontPath = args[i]
		case strings.HasPrefix(a, "--font="):
			opts.FontPath = a[len("--font="):]
			if opts.FontPath == "" {
				fmt.Fprintln(os.Stderr, "--font requires a path")
				return 2
			}
		case a == "-j" || a == "--concurrency":
			i++
			n, err := strconv.Atoi(argAt(args, i))
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "-j requires a positive integer")
				return 2
			}
			opts.Concurrency = n
		case strings.HasPrefix(a, "-j"):
			n, err := strconv.Atoi(a[2:])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "-j requires a positive integer")
				return 2
			}
			opts.Concurrency = n
		case a == "--version" || a == "-V":
			fmt.Printf("astrogopng %s\n", version)
			return 0
		case a == "--help" || a == "-h" || a == "/?":
			fmt.Printf(usage, version)
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "Unknown option: %s\n", a)
			return 2
		default:
			if isFile(a) {
				opts.Files = append(opts.Files, a)
			} else if inputDir == "" && len(opts.Files) == 0 {
				inputDir = a
			} else if outputDir == "" {
				outputDir = a
			} else {
				fmt.Fprintf(os.Stderr, "Unexpected argument: %s\n", a)
				return 2
			}
		}
	}

	if len(opts.Files) > 0 && outputDir == "" {
		outputDir, inputDir = inputDir, ""
	}
	if inputDir == "" {
		inputDir = "."
	}
	opts.InputDir = inputDir
	opts.OutputDir = outputDir

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	verb := "Converted"
	if opts.PNGOnly {
		verb = "Processed"
	}

	summary, err := batch.Run(ctx, opts, func(p batch.Progress) {
		rel := filepath.ToSlash(p.Rel)
		switch p.Status {
		case batch.StatusOK:
			if p.Label != "" {
				fmt.Printf("OK    %s  ->  %s\n", rel, p.Label)
			} else {
				fmt.Printf("OK    %s\n", rel)
			}
		case batch.StatusSkipped:
			fmt.Printf("SKIP  %s\n", rel)
		case batch.StatusFailed:
			fmt.Printf("ERROR %s: %s\n", rel, p.Err)
		}
		if p.Note != "" {
			fmt.Printf("      note: %s\n", p.Note)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if summary.Total == 0 {
		fmt.Printf("No %s files found.\n", opts.InputKind())
		return 0
	}

	fmt.Printf("\n%s: %d   Skipped: %d   Failed: %d", verb, summary.Converted, summary.Skipped, summary.Failed)
	if summary.Cancelled {
		fmt.Print("   (cancelled)")
	}
	fmt.Println()
	for _, w := range summary.Warnings {
		fmt.Printf("WARNING: %s\n", w)
	}

	if summary.Failed > 0 {
		return 1
	}
	return 0
}

func argAt(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
