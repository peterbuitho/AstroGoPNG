//go:build !windows && !linux

package main

import "fyne.io/fyne/v2"

// macOS and the rest have nothing to install; drop files onto the window.
func shellSection(_ *ui) fyne.CanvasObject { return nil }
