//go:build windows

package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows/registry"
)

const (
	verb  = "astrogopng"
	label = "Convert to PNG with astrogopng"
)

func keyPath(ext string) string {
	return fmt.Sprintf(`Software\Classes\SystemFileAssociations\.%s\shell\%s`, ext, verb)
}

func integrationInstalled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath(integrationExts[0])+`\command`, registry.READ)
	if err != nil {
		return false
	}
	_ = k.Close()
	return true
}

func integrationInstall() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	command := fmt.Sprintf(`"%s" "%%1"`, exe)
	for _, ext := range integrationExts {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath(ext), registry.WRITE)
		if err != nil {
			return fmt.Errorf(".%s: %w", ext, err)
		}
		_ = k.SetStringValue("", label)
		_ = k.SetStringValue("Icon", exe)
		_ = k.SetStringValue("MultiSelectModel", "Player")
		ck, _, err := registry.CreateKey(k, "command", registry.WRITE)
		if err != nil {
			_ = k.Close()
			return err
		}
		_ = ck.SetStringValue("", command)
		_ = ck.Close()
		_ = k.Close()
	}
	return nil
}

func integrationUninstall() error {
	for _, ext := range integrationExts {
		_ = registry.DeleteKey(registry.CURRENT_USER, keyPath(ext)+`\command`)
		_ = registry.DeleteKey(registry.CURRENT_USER, keyPath(ext))
	}
	return nil
}

func shellSection(u *ui) fyne.CanvasObject {
	status := widget.NewLabel("")
	var btn *widget.Button
	refresh := func() {
		if integrationInstalled() {
			btn.SetText("Remove from Explorer right-click menu")
		} else {
			btn.SetText("Add to Explorer right-click menu")
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
