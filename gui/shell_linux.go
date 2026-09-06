//go:build linux

package main

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func dataHome() string {
	if x := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(x) {
		return x
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func desktopFile() string {
	return filepath.Join(dataHome(), "applications", "astrogopng.desktop")
}

func integrationInstalled() bool {
	_, err := os.Stat(desktopFile())
	return err == nil
}

func integrationInstall() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(desktopFile()), 0o755); err != nil {
		return err
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=astrogopng\n" +
		"Comment=Convert XISF / FITS astrophotos to PNG\n" +
		"Exec=\"" + exe + "\" %F\n" +
		"Icon=astrogopng\n" +
		"Terminal=false\n" +
		"Categories=Graphics;Photography;\n" +
		"MimeType=image/fits;application/fits;image/x-fits;application/x-xisf;\n"
	return os.WriteFile(desktopFile(), []byte(content), 0o644)
}

func integrationUninstall() error {
	return os.Remove(desktopFile())
}

func shellSection(u *ui) fyne.CanvasObject {
	status := widget.NewLabel("")
	var btn *widget.Button
	refresh := func() {
		if integrationInstalled() {
			btn.SetText("Remove \"Open with\" desktop entry")
		} else {
			btn.SetText("Install \"Open with astrogopng\" desktop entry")
		}
	}
	btn = widget.NewButton("", func() {
		var err error
		if integrationInstalled() {
			err = integrationUninstall()
		} else {
			err = integrationInstall()
		}
		if err != nil {
			status.SetText("Failed: " + err.Error())
		} else {
			status.SetText("Done.")
		}
		refresh()
	})
	refresh()
	return container.NewVBox(container.NewHBox(btn, status))
}
