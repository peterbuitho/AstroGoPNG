// Command astrogopng-gui is the desktop front-end for the AstroGoPNG
// converter. The batch runs on a worker pool (see internal/batch) while the
// window streams progress; the UI thread never blocks on conversion.
package main

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/peterbuitho/AstroGoPNG/internal/batch"
)

var version = "dev"

type ui struct {
	win fyne.Window

	inputEntry, outputEntry, fontEntry *widget.Entry
	recursive, overwrite               *widget.Check
	pngOnly, resize4k, lookup          *widget.Check

	files    []string
	filesBox *fyne.Container

	convertBtn *widget.Button
	progress   *widget.ProgressBar
	statusLbl  *widget.RichText

	mu       sync.Mutex
	logLines [][]widget.RichTextSegment
	logList  *widget.List

	cancel context.CancelFunc
}

func main() {
	// Files / folders passed on the command line — Explorer's right-click
	// verb / "Open with" launches one process per selected file.
	initial := os.Args[1:]

	// Single-instance: the first launch owns the window; every later launch
	// hands it its paths and exits, so a multi-selection is one file list in
	// one window.
	ln, primary := claim(initial)
	if !primary {
		return
	}

	a := app.NewWithID("io.github.peterbuitho.astrogopng")
	w := a.NewWindow("AstroGoPNG " + version)
	w.Resize(fyne.NewSize(820, 720))

	u := &ui{win: w}
	u.build()
	w.SetContent(u.root())

	for _, arg := range initial {
		u.addPath(arg)
	}

	serve(ln, func(paths []string) {
		fyne.Do(func() {
			for _, p := range paths {
				u.addPath(p)
			}
			w.RequestFocus()
		})
	})

	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		for _, uri := range uris {
			u.addPath(uri.Path())
		}
	})

	if shot := os.Getenv("ASTRO_SHOT"); shot != "" {
		auto := os.Getenv("ASTRO_AUTOCONVERT") != ""
		go func() {
			if auto {
				time.Sleep(400 * time.Millisecond)
				fyne.DoAndWait(u.onConvert)
				time.Sleep(6 * time.Second)
			} else {
				time.Sleep(1500 * time.Millisecond)
			}
			fyne.DoAndWait(func() {
				img := w.Canvas().Capture()
				if f, err := os.Create(shot); err == nil {
					_ = png.Encode(f, img)
					_ = f.Close()
				}
				a.Quit()
			})
		}()
	}

	w.ShowAndRun()
}

func (u *ui) build() {
	u.inputEntry = widget.NewEntry()
	u.inputEntry.SetPlaceHolder("current folder")
	u.outputEntry = widget.NewEntry()
	u.outputEntry.SetPlaceHolder("same as input")
	u.fontEntry = widget.NewEntry()
	u.fontEntry.SetPlaceHolder("bundled DejaVu Sans Condensed Bold")

	u.recursive = widget.NewCheck("Recurse into subfolders", nil)
	u.overwrite = widget.NewCheck("Overwrite existing PNGs", nil)
	u.pngOnly = widget.NewCheck("PNG only (no XISF/FITS conversion)", nil)
	u.resize4k = widget.NewCheck("Resize to 3840×2160 and stamp object name", nil)
	u.resize4k.Checked = true
	u.lookup = widget.NewCheck("Stamp object name (SIMBAD lookup); off = file name", nil)
	u.lookup.Checked = true

	u.filesBox = container.NewVBox()

	u.progress = widget.NewProgressBar()
	u.statusLbl = widget.NewRichText()
	u.statusLbl.Wrapping = fyne.TextWrapWord
	u.convertBtn = widget.NewButton("Convert", u.onConvert)
	u.convertBtn.Importance = widget.HighImportance

	u.logList = widget.NewList(
		func() int { u.mu.Lock(); defer u.mu.Unlock(); return len(u.logLines) },
		func() fyne.CanvasObject {
			return widget.NewRichText()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			u.mu.Lock()
			defer u.mu.Unlock()
			if i < len(u.logLines) {
				rt := o.(*widget.RichText)
				rt.Segments = u.logLines[i]
				rt.Refresh()
			}
		},
	)
}

func (u *ui) root() fyne.CanvasObject {
	browseFolder := func(target *widget.Entry) *widget.Button {
		return widget.NewButton("Browse…", func() {
			dialog.ShowFolderOpen(func(list fyne.ListableURI, err error) {
				if err == nil && list != nil {
					target.SetText(list.Path())
				}
			}, u.win)
		})
	}
	browseFont := widget.NewButton("Browse…", func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err == nil && rc != nil {
				u.fontEntry.SetText(rc.URI().Path())
				_ = rc.Close()
			}
		}, u.win)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".ttf", ".otf"}))
		fd.Show()
	})

	addFiles := widget.NewButton("Add file…", func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err == nil && rc != nil {
				u.addPath(rc.URI().Path())
				_ = rc.Close()
			}
		}, u.win)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".xisf", ".fits", ".fit", ".fts", ".png"}))
		fd.Show()
	})
	clearFiles := widget.NewButton("Clear", func() {
		u.files = nil
		u.refreshFiles()
	})

	checks := container.NewGridWithColumns(2,
		u.recursive, u.overwrite, u.pngOnly, u.resize4k, u.lookup)

	form := container.NewVBox(
		widget.NewLabelWithStyle("XISF / FITS to PNG batch converter", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		labeledRow("Input folder", u.inputEntry, browseFolder(u.inputEntry)),
		container.NewHBox(addFiles, clearFiles,
			widget.NewLabelWithStyle("or drop files / a folder onto this window", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})),
		u.filesBox,
		labeledRow("Output folder", u.outputEntry, browseFolder(u.outputEntry)),
		labeledRow("Stamp font", u.fontEntry, browseFont),
		checks,
	)
	if s := shellSection(u); s != nil {
		form.Add(s)
	}

	u.refreshFiles()

	controls := container.NewVBox(
		widget.NewSeparator(),
		u.convertBtn,
		u.progress,
		u.statusLbl,
	)

	logArea := container.NewBorder(
		widget.NewLabelWithStyle("Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		u.logList, // widget.List scrolls itself
	)

	// Form at natural height on top, Convert / progress / status pinned at
	// the bottom, the log fills everything in between and scrolls.
	return container.NewBorder(form, controls, nil, nil, logArea)
}

func (u *ui) refreshFiles() {
	u.filesBox.RemoveAll()
	if len(u.files) == 0 {
		u.filesBox.Refresh()
		return
	}
	u.filesBox.Add(widget.NewLabelWithStyle(
		fmt.Sprintf("%d file(s) selected:", len(u.files)),
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	for _, f := range u.files {
		l := widget.NewLabel("  " + filepath.Base(f))
		l.TextStyle = fyne.TextStyle{Monospace: true}
		u.filesBox.Add(l)
	}
	u.filesBox.Refresh()
}

func labeledRow(label string, entry *widget.Entry, browse *widget.Button) fyne.CanvasObject {
	l := widget.NewLabel(label)
	return container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(110, l.MinSize().Height), l),
		browse, entry)
}

func (u *ui) addPath(p string) {
	if isDir(p) {
		u.inputEntry.SetText(p)
		return
	}
	for _, f := range u.files {
		if f == p {
			return
		}
	}
	u.files = append(u.files, p)
	u.refreshFiles()
}

func (u *ui) options() batch.Options {
	return batch.Options{
		InputDir:  orDefault(u.inputEntry.Text, "."),
		OutputDir: u.outputEntry.Text,
		Recursive: u.recursive.Checked,
		Overwrite: u.overwrite.Checked,
		Resize4K:  u.resize4k.Checked,
		PNGOnly:   u.pngOnly.Checked,
		Lookup:    u.lookup.Checked,
		FontPath:  u.fontEntry.Text,
		Files:     append([]string(nil), u.files...),
	}
}

func (u *ui) onConvert() {
	if u.cancel != nil { // running -> cancel
		u.cancel()
		return
	}
	opts := u.options()

	u.mu.Lock()
	u.logLines = nil
	u.mu.Unlock()
	u.logList.Refresh()
	u.progress.SetValue(0)
	u.setStatus()

	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	u.convertBtn.SetText("Cancel")

	go func() {
		summary, err := batch.Run(ctx, opts, func(p batch.Progress) {
			lines := formatProgress(p)
			u.mu.Lock()
			u.logLines = append(u.logLines, lines...)
			u.mu.Unlock()
			frac := 0.0
			if p.Total > 0 {
				frac = float64(p.Index) / float64(p.Total)
			}
			fyne.Do(func() {
				u.progress.SetValue(frac)
				u.logList.Refresh()
				u.logList.ScrollToBottom()
			})
		})
		fyne.Do(func() {
			u.cancel = nil
			u.convertBtn.SetText("Convert")
			u.progress.SetValue(1)
			switch {
			case err != nil:
				u.setStatus(plainSeg("Error: "+err.Error(), theme.ColorNameError))
			case summary.Total == 0:
				u.setStatus(plainSeg("No "+opts.InputKind()+" files found.", theme.ColorNameForeground))
			default:
				verb := "Converted"
				if opts.PNGOnly {
					verb = "Processed"
				}
				summaryColor := theme.ColorNameForeground
				if summary.Failed > 0 {
					summaryColor = theme.ColorNameError
				}
				s := fmt.Sprintf("%s: %d   Skipped: %d   Failed: %d",
					verb, summary.Converted, summary.Skipped, summary.Failed)
				if summary.Cancelled {
					s += "   (cancelled)"
				}
				segments := []widget.RichTextSegment{plainSeg(s, summaryColor)}
				for _, w := range summary.Warnings {
					segments = append(segments, plainSeg("\n⚠ "+w, theme.ColorNameWarning))
				}
				u.setStatus(segments...)
			}
		})
	}()
}

// monoSeg/plainSeg are RichText segments colored by theme color name (so the
// text reads correctly in both light and dark themes), monospace for the log
// and proportional for the status line, matching each widget's prior style.
func monoSeg(text string, colorName fyne.ThemeColorName) widget.RichTextSegment {
	return &widget.TextSegment{
		Text:  text,
		Style: widget.RichTextStyle{ColorName: colorName, TextStyle: fyne.TextStyle{Monospace: true}},
	}
}

func plainSeg(text string, colorName fyne.ThemeColorName) widget.RichTextSegment {
	return &widget.TextSegment{Text: text, Style: widget.RichTextStyle{ColorName: colorName}}
}

func (u *ui) setStatus(segments ...widget.RichTextSegment) {
	u.statusLbl.Segments = segments
	u.statusLbl.Refresh()
}

func formatProgress(p batch.Progress) [][]widget.RichTextSegment {
	var out [][]widget.RichTextSegment
	switch p.Status {
	case batch.StatusOK:
		line := []widget.RichTextSegment{
			monoSeg("OK    ", theme.ColorNameSuccess),
			monoSeg(p.Rel, theme.ColorNameForeground),
		}
		if p.Label != "" {
			line = append(line,
				monoSeg("  →  ", theme.ColorNameForeground),
				monoSeg(p.Label, theme.ColorNamePrimary))
		}
		out = append(out, line)
	case batch.StatusSkipped:
		out = append(out, []widget.RichTextSegment{
			monoSeg("SKIP  ", theme.ColorNameDisabled),
			monoSeg(p.Rel, theme.ColorNameForeground),
		})
	case batch.StatusFailed:
		out = append(out, []widget.RichTextSegment{
			monoSeg("ERROR ", theme.ColorNameError),
			monoSeg(p.Rel+": "+p.Err, theme.ColorNameError),
		})
	}
	if p.Note != "" {
		out = append(out, []widget.RichTextSegment{
			monoSeg("      note: ", theme.ColorNameForeground),
			monoSeg(p.Note, theme.ColorNameWarning),
		})
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
