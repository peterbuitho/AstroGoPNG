# AstroGoPNG

Batch-convert **XISF** (PixInsight) and **FITS** astronomical images to
**PNG**, optionally resized to 4K with the object's name and catalogue info
stamped in the corner ("Andromeda Galaxy (M 31)" with NGC/IC ids, type and
coordinates underneath, looked up from SIMBAD).

A **Go** port of [`xisf2png`](https://github.com/peterbuitho/xisf2png) /
[`Astro2PNG`](https://github.com/peterbuitho/Astro2PNG). Files are converted
**in parallel** — a pool of workers (one per CPU by default) each runs the
decode → stretch → resize → stamp → encode pipeline, so a folder of subs
finishes in a fraction of the wall-clock time. The online object lookup is
shared across workers with a cache, so 300 subs of one target still cost only
one or two SIMBAD requests.

Each image's full data range is linearly scaled to 0–255 (a plain min/max
stretch). Only the first image in a file is converted. Mono and RGB are
supported.

## Install / build

Requires [Go 1.27+](https://go.dev/dl/). No C toolchain — everything is pure
Go (`CGO_ENABLED=0`), so it cross-compiles to every desktop platform from one
machine.

```
go build ./cmd/astrogopng          # -> ./astrogopng
go install github.com/peterbuitho/AstroGoPNG/cmd/astrogopng@latest
go test ./...
```

Prebuilt binaries for Windows, macOS (Intel + Apple Silicon) and Linux
(x86-64 + ARM64) are attached to each
[GitHub Release](https://github.com/peterbuitho/AstroGoPNG/releases).

## Command line

```
astrogopng [input_dir] [output_dir] [--recursive|-r] [--overwrite] [--resize4k] [--filename] [-j N]
astrogopng [input_dir] [output_dir] --png-only [--recursive|-r] [--overwrite] [--filename]
astrogopng <file>... [output_dir] [--overwrite] [--resize4k] [--filename]
```

Every `.xisf`, `.fits`, `.fit` and `.fts` file found is converted. If
`input_dir` is omitted, the current folder is used. If `output_dir` is
omitted, PNGs are written next to their source files. Explicit files may be
given instead of a folder; `.png` files are only resized/stamped.

| Option                 | Meaning |
| ---------------------- | ------- |
| `-r`, `--recursive`    | Recurse into subfolders; the output tree mirrors the input. |
| `--overwrite`          | Overwrite existing `.png` files (default: skip). |
| `--resize4k`           | Scale each PNG (aspect kept) to cover 3840×2160, centre-crop to exactly 3840×2160, and stamp the object name bottom-right — identified from the header `OBJECT`, a catalogue id in the file name (`M31`, `NGC_7000`, `Sh2-155`, …) and/or the image coordinates, resolved via CDS Sesame / SIMBAD and cross-checked. Falls back to the file name offline. |
| `--filename`           | Stamp the plain file name: ignore the header `OBJECT`, never go online. Aliases: `--no-lookup`, `--offline`. |
| `--png-only`           | Skip XISF/FITS conversion: pick up existing `.png` files and only run the resize step (implies `--resize4k`). |
| `--font <file>`        | A `.ttf` / `.otf` font file for the stamp. Default: bundled DejaVu Sans Condensed Bold. |
| `-j`, `--concurrency N`| Files to convert in parallel. Default: number of CPUs, capped at 8. |
| `-V`, `--version`      | Print the version. |
| `-h`, `--help`         | Show help. |

Per-file output lines: `OK <path>`, `SKIP <path>`, `ERROR <path>: <reason>`
(in completion order — parallel workers may finish out of order). Exit code is
`1` if any file failed, `2` for a usage error, `0` otherwise. `Ctrl+C` stops
after the in-flight files.

## Format support

**XISF** (monolithic `.xisf`, version 1.0): `UInt8/16/32/64`, `Float32/64`;
planar and interleaved; little- and big-endian; `attachment`, `embedded`,
`inline` (base64 / hex); `zlib`, `lz4`, `lz4hc`, `zstd` compression, with or
without byte-shuffle.

**FITS** (`.fits`, `.fit`, `.fts`, image in the primary HDU): `BITPIX` 8, 16,
32, 64 (with `BZERO`/`BSCALE`), −32, −64; `NAXIS` 2 or 3 with `NAXIS3` = 1 or
3; `ROWORDER`. Not supported: images in extensions, tile-compressed `.fz`,
`.fits.gz`.

## Desktop GUI

`AstroGoPNG` also ships a [Fyne](https://fyne.io) desktop app —
`astrogopng-gui` — with the same engine: pick folders / drop files, set the
options, hit **Convert**, and watch the parallel batch stream a log and
progress bar. On Windows it can add a *"Convert to PNG with astrogopng"*
Explorer right-click entry; on Linux it installs an *Open with* `.desktop`
entry.

The GUI is a **separate Go module** (`gui/`) so the CLI stays pure-Go and
dependency-light. It needs a C compiler (Fyne uses OpenGL):

```
cd gui
go build -ldflags -H=windowsgui .   # Windows: -H=windowsgui suppresses the console window
CC="zig cc" go build .              # if you have Zig but no gcc/clang
```

Prebuilt GUI binaries for Windows, macOS and Linux are in the
[Releases](https://github.com/peterbuitho/AstroGoPNG/releases) alongside the
CLI.

## Status

Feature parity with the original for both the CLI and the GUI. Not yet ported:
the single-instance file hand-off and the optional user-names file.

## Dependencies

Almost everything is the Go standard library (`image/png`, `compress/zlib`,
`encoding/xml`, `net/http`, …). External:

| Module | Purpose |
| --- | --- |
| `github.com/klauspost/compress/zstd` | XISF zstd codec (pure Go) |
| `golang.org/x/image` | Catmull-Rom resize, TrueType stamp rendering |
| `golang.org/x/sync/singleflight` | collapse duplicate concurrent SIMBAD lookups |

The stamp font, DejaVu Sans Condensed Bold, is embedded; its licence is in
[`assets/fonts/LICENSE-DejaVu.txt`](assets/fonts/LICENSE-DejaVu.txt).
