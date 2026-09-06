package main

import "os"

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// extensions offered the right-click / "Open with" integration.
var integrationExts = []string{"xisf", "fits", "fit", "fts"}
