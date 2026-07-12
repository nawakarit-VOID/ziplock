// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"embed"
	"fmt"
	"os"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// โหลด icon
func loadIcon(size int) fyne.Resource {
	var file string

	switch {
	case size >= 512:
		file = "assets/icons/icon-512.png" ///ที่อยู่
	case size >= 256:
		file = "assets/icons/icon-256.png"
	case size >= 128:
		file = "assets/icons/icon-128.png"
	default:
		file = "assets/icons/icon-64.png"
	}

	data, err := iconFS.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot load icon %s: %v\n", file, err)
		return fyne.NewStaticResource("missing-icon", nil)
	}
	if len(data) == 0 {
		fmt.Fprintf(os.Stderr, "warning: icon %s is empty\n", file)
		return fyne.NewStaticResource("empty-icon", nil)
	}
	return fyne.NewStaticResource(file, data)
}

//go:embed assets/icons/*
var iconFS embed.FS

//go:embed assets/font/Itim-Regular.ttf
var fontItim []byte
var myFont = fyne.NewStaticResource("Itim-Regular.ttf", fontItim)

func runOnUI(fn func()) {
	fyne.Do(fn)
}

func runGUI() {
	a := app.NewWithID("com.nawakarit.ziplock")
	a.Settings().SetTheme(&MyTheme{})
	icon := loadIcon(64)
	a.SetIcon(icon)

	inputPath := widget.NewEntry()
	outputPath := widget.NewEntry()
	password := widget.NewPasswordEntry()
	status := widget.NewMultiLineEntry()
	w := a.NewWindow("ziplock : โปรแกรมบีบอัดไฟล์")
	w.Resize(fyne.NewSize(760, 520))
	w.SetIcon(icon)

	inputPath.SetPlaceHolder("เลือกไฟล์ต้นทาง")
	outputPath.SetPlaceHolder("กำหนดไฟล์ปลายทาง")
	password.SetPlaceHolder("ใส่รหัสผ่าน")
	status.SetText("พร้อมใช้งาน")
	status.Disable()

	appendStatus := func(msg string) {
		runOnUI(func() {
			if current := status.Text; current == "" {
				status.SetText(msg)
			} else {
				status.SetText(current + "\n" + msg)
			}
		})
	}

	browseInput := widget.NewButton("เลือกไฟล์", func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			path := r.URI().Path()
			_ = r.Close()
			runOnUI(func() {
				inputPath.SetText(path)
			})
		}, w)
	})

	browseOutput := widget.NewButton("เลือกปลายทาง", func() {
		dialog.ShowFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			path := wc.URI().Path()
			_ = wc.Close()
			runOnUI(func() {
				outputPath.SetText(path)
			})
		}, w)
	})

	packBtn := widget.NewButton("Pack", func() {
		input := inputPath.Text
		output := outputPath.Text
		pass := password.Text
		if input == "" || output == "" || pass == "" {
			runOnUI(func() {
				dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			})
			return
		}
		appendStatus("กำลัง pack...")
		go func() {
			err := pack(input, output, pass)
			runOnUI(func() {
				if err != nil {
					appendStatus("Error: " + err.Error())
					dialog.ShowError(err, w)
					return
				}
				appendStatus("Pack สำเร็จ")
				dialog.ShowInformation("ziplock", "Pack สำเร็จ", w)
			})
		}()
	})

	unpackBtn := widget.NewButton("Unpack", func() {
		input := inputPath.Text
		output := outputPath.Text
		pass := password.Text
		if input == "" || output == "" || pass == "" {
			runOnUI(func() {
				dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			})
			return
		}
		appendStatus("กำลัง unpack...")
		go func() {
			err := unpack(input, output, pass)
			runOnUI(func() {
				if err != nil {
					appendStatus("Error: " + err.Error())
					dialog.ShowError(err, w)
					return
				}
				appendStatus("Unpack สำเร็จ")
				dialog.ShowInformation("ziplock", "Unpack สำเร็จ", w)
			})
		}()
	})

	platformNote := widget.NewLabel("Desktop app สำหรับ " + runtime.GOOS)
	form := container.NewVBox(
		widget.NewLabel("ziplock"),
		platformNote,
		container.NewBorder(nil, nil, nil, browseInput, inputPath),
		container.NewBorder(nil, nil, nil, browseOutput, outputPath),
		password,
		container.NewHBox(packBtn, unpackBtn),
		widget.NewLabel("สถานะ"),
		status,
	)

	w.SetContent(container.NewPadded(form))
	w.ShowAndRun()
}
