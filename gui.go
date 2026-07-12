// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
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

	packInputPath := widget.NewEntry()
	packOutputPath := widget.NewEntry()
	packPassword := widget.NewPasswordEntry()
	unpackInputPath := widget.NewEntry()
	unpackOutputPath := widget.NewEntry()
	unpackPassword := widget.NewPasswordEntry()
	status := widget.NewMultiLineEntry()
	w := a.NewWindow("ziplock : โปรแกรมบีบอัดไฟล์")
	w.Resize(fyne.NewSize(760, 520))
	w.SetIcon(icon)

	packInputPath.SetPlaceHolder("เลือกไฟล์หรือโฟลเดอร์ต้นทาง")
	packOutputPath.SetPlaceHolder("เลือกโฟลเดอร์ปลายทาง หรือไฟล์ .myz")
	packPassword.SetPlaceHolder("ใส่รหัสผ่านสำหรับ pack")
	unpackInputPath.SetPlaceHolder("เลือกไฟล์ .myz")
	unpackOutputPath.SetPlaceHolder("เลือกโฟลเดอร์ปลายทาง")
	unpackPassword.SetPlaceHolder("ใส่รหัสผ่านสำหรับ unpack")
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

	browsePackInput := widget.NewButton("เลือกต้นทาง", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err == nil && lu != nil {
				runOnUI(func() {
					packInputPath.SetText(lu.Path())
				})
				return
			}
			dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil || r == nil {
					return
				}
				path := r.URI().Path()
				_ = r.Close()
				runOnUI(func() {
					packInputPath.SetText(path)
				})
			}, w)
		}, w)
	})

	browsePackOutput := widget.NewButton("เลือกปลายทาง", func() {
		dialog.ShowFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			path := wc.URI().Path()
			_ = wc.Close()
			runOnUI(func() {
				packOutputPath.SetText(path)
			})
		}, w)
	})

	browseUnpackInput := widget.NewButton("เลือกไฟล์ .myz", func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			path := r.URI().Path()
			_ = r.Close()
			runOnUI(func() {
				unpackInputPath.SetText(path)
			})
		}, w)
	})

	browseUnpackOutput := widget.NewButton("เลือกโฟลเดอร์", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			path := lu.Path()
			runOnUI(func() {
				unpackOutputPath.SetText(path)
			})
		}, w)
	})

	packBtn := widget.NewButton("Pack", func() {
		input := packInputPath.Text
		output := packOutputPath.Text
		pass := packPassword.Text
		if input == "" || output == "" || pass == "" {
			runOnUI(func() {
				dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			})
			return
		}
		if info, err := os.Stat(input); err == nil && !info.IsDir() && filepath.Ext(output) == "" {
			output += ".myz"
			runOnUI(func() {
				packOutputPath.SetText(output)
			})
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
		input := unpackInputPath.Text
		output := unpackOutputPath.Text
		pass := unpackPassword.Text
		if input == "" || output == "" || pass == "" {
			runOnUI(func() {
				dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			})
			return
		}
		if info, err := os.Stat(output); err == nil && !info.IsDir() {
			runOnUI(func() {
				dialog.ShowInformation("ziplock", "Unpack ต้องเลือกโฟลเดอร์ปลายทาง", w)
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
	packSection := container.NewVBox(
		widget.NewLabel("Pack Archive"),
		container.NewBorder(nil, nil, nil, browsePackInput, packInputPath),
		container.NewBorder(nil, nil, nil, browsePackOutput, packOutputPath),
		packPassword,
		packBtn,
	)

	unpackSection := container.NewVBox(
		widget.NewLabel("Unpack Archive"),
		container.NewBorder(nil, nil, nil, browseUnpackInput, unpackInputPath),
		container.NewBorder(nil, nil, nil, browseUnpackOutput, unpackOutputPath),
		unpackPassword,
		unpackBtn,
	)

	form := container.NewVBox(
		widget.NewLabel("ziplock"),
		platformNote,
		packSection,
		unpackSection,
		widget.NewLabel("สถานะ"),
		status,
	)

	w.SetContent(container.NewPadded(form))
	w.ShowAndRun()
}
