// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
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

// autoPackFileName returns just the filename (no directory) for the output .ziplock
func autoPackFileName(inputPath string) string {
	if inputPath == "" {
		return ""
	}
	info, err := os.Stat(inputPath)
	if err == nil && info.IsDir() {
		return filepath.Base(filepath.Clean(inputPath)) + ".ziplock"
	}
	base := filepath.Base(inputPath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + ".ziplock"
}

// autoPackOutputDir returns the directory of the input path as the default output dir
func autoPackOutputDir(inputPath string) string {
	if inputPath == "" {
		return ""
	}
	info, err := os.Stat(inputPath)
	if err == nil && info.IsDir() {
		return filepath.Dir(filepath.Clean(inputPath))
	}
	return filepath.Dir(inputPath)
}

/*
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
	packOutputPath.SetPlaceHolder("ปลายทาง .ziplock จะถูกตั้งชื่ออัตโนมัติ")
	packPassword.SetPlaceHolder("ใส่รหัสผ่านสำหรับ pack")
	unpackInputPath.SetPlaceHolder("เลือกไฟล์ .ziplock")
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
				path := lu.Path()
				runOnUI(func() {
					packInputPath.SetText(path)
					packOutputPath.SetText(autoPackOutputName(path))
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
					packOutputPath.SetText(autoPackOutputName(path))
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

	browseUnpackInput := widget.NewButton("เลือกไฟล์ .ziplock", func() {
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
		if filepath.Ext(output) == "" {
			output = autoPackOutputName(input)
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

	//platformNote := widget.NewLabel("Desktop app สำหรับ " + runtime.GOOS)
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
		//widget.NewLabel("ziplock"),
		//platformNote,
		packSection,
		unpackSection,
		widget.NewLabel("สถานะ"),
		status,
	)

	w.SetContent(container.NewPadded(form))
	w.ShowAndRun()
}
*/
// NOTE: เพิ่ม import เหล่านี้ที่ยังไม่มีในไฟล์เดิม
//   "fyne.io/fyne/v2/theme"
// ส่วน pack(), unpack(), loadIcon(), runOnUI(), MyTheme, autoPackOutputName()
// ใช้ของเดิมที่มีอยู่แล้วในโปรเจกต์ ไม่ต้องแก้ไข

func runGUI() {
	a := app.NewWithID("com.nawakarit.ziplock")
	a.Settings().SetTheme(&MyTheme{})
	icon := loadIcon(64)
	a.SetIcon(icon)

	w := a.NewWindow("ziplock : โปรแกรมบีบอัดไฟล์")
	w.Resize(fyne.NewSize(820, 640))
	w.SetIcon(icon)

	// ---------- inputs ----------
	packInputPath := widget.NewEntry()
	packOutputDir := widget.NewEntry()  // โฟลเดอร์ปลายทาง
	packOutputName := widget.NewEntry() // ชื่อไฟล์
	packPassword := widget.NewPasswordEntry()
	unpackInputPath := widget.NewEntry()
	unpackOutputPath := widget.NewEntry()
	unpackPassword := widget.NewPasswordEntry()

	packInputPath.SetPlaceHolder("เลือกไฟล์หรือโฟลเดอร์ต้นทาง")
	packOutputDir.SetPlaceHolder("เลือกโฟลเดอร์ปลายทาง")
	packOutputName.SetPlaceHolder("ชื่อไฟล์ .ziplock (ตั้งอัตโนมัติ แก้ไขได้)")
	packPassword.SetPlaceHolder("ใส่รหัสผ่านสำหรับ pack")
	unpackInputPath.SetPlaceHolder("เลือกไฟล์ .ziplock")
	unpackOutputPath.SetPlaceHolder("เลือกโฟลเดอร์ปลายทาง")
	unpackPassword.SetPlaceHolder("ใส่รหัสผ่านสำหรับ unpack")

	// ---------- status + progress ----------
	status := widget.NewMultiLineEntry()
	status.Wrapping = fyne.TextWrapWord
	status.SetText("พร้อมใช้งาน")
	status.Disable()

	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	appendStatus := func(msg string) {
		runOnUI(func() {
			if current := status.Text; current == "" {
				status.SetText(msg)
			} else {
				status.SetText(current + "\n" + msg)
			}
		})
	}

	var packBtn, unpackBtn *widget.Button
	setBusy := func(busy bool) {
		runOnUI(func() {
			if busy {
				progress.Show()
				packBtn.Disable()
				unpackBtn.Disable()
			} else {
				progress.Hide()
				packBtn.Enable()
				unpackBtn.Enable()
			}
		})
	}

	// ---------- browse buttons ----------
	browsePackInput := widget.NewButtonWithIcon("เลือกต้นทาง", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err == nil && lu != nil {
				path := lu.Path()
				runOnUI(func() {
					packInputPath.SetText(path)
					// ตั้งชื่อไฟล์อัตโนมัติ แต่เฉพาะถ้าผู้ใช้ยังไม่ได้แก้ไข
					if packOutputDir.Text == "" {
						packOutputDir.SetText(autoPackOutputDir(path))
					}
					packOutputName.SetText(autoPackFileName(path))
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
					if packOutputDir.Text == "" {
						packOutputDir.SetText(autoPackOutputDir(path))
					}
					packOutputName.SetText(autoPackFileName(path))
				})
			}, w)
		}, w)
	})

	// เลือกโฟลเดอร์ปลายทาง (ไม่สร้างไฟล์ใดๆ)
	browsePackOutputDir := widget.NewButtonWithIcon("เลือกโฟลเดอร์", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			path := lu.Path()
			runOnUI(func() {
				packOutputDir.SetText(path)
			})
		}, w)
	})

	browseUnpackInput := widget.NewButtonWithIcon("เลือกไฟล์ .ziplock", theme.FileIcon(), func() {
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

	browseUnpackOutput := widget.NewButtonWithIcon("เลือกโฟลเดอร์", theme.FolderOpenIcon(), func() {
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

	// ---------- main actions ----------
	packBtn = widget.NewButtonWithIcon("Pack", theme.UploadIcon(), func() {
		input := packInputPath.Text
		outDir := strings.TrimSpace(packOutputDir.Text)
		outName := strings.TrimSpace(packOutputName.Text)
		pass := packPassword.Text

		if input == "" || pass == "" {
			dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			return
		}

		// ถ้ายังไม่มีชื่อไฟล์ ให้ตั้งชื่ออัตโนมัติ
		if outName == "" {
			outName = autoPackFileName(input)
			runOnUI(func() { packOutputName.SetText(outName) })
		}
		// ถ้ายังไม่มีโฟลเดอร์ปลายทาง ให้ใช้โฟลเดอร์เดียวกับต้นทาง
		if outDir == "" {
			outDir = autoPackOutputDir(input)
			runOnUI(func() { packOutputDir.SetText(outDir) })
		}
		// ตรวจสอบนามสกุลไฟล์
		if filepath.Ext(outName) != ".ziplock" {
			outName = strings.TrimSuffix(outName, filepath.Ext(outName)) + ".ziplock"
			runOnUI(func() { packOutputName.SetText(outName) })
		}

		// รวม path ที่แน่นอน
		output := filepath.Join(outDir, outName)

		appendStatus(fmt.Sprintf("กำลัง pack → %s", output))
		setBusy(true)
		go func() {
			err := pack(input, output, pass)
			setBusy(false)
			runOnUI(func() {
				if err != nil {
					appendStatus("Error: " + err.Error())
					dialog.ShowError(err, w)
					return
				}
				appendStatus("Pack สำเร็จ: " + output)
				dialog.ShowInformation("ziplock", "Pack สำเร็จ", w)
			})
		}()
	})
	packBtn.Importance = widget.HighImportance

	unpackBtn = widget.NewButtonWithIcon("Unpack", theme.DownloadIcon(), func() {
		input := unpackInputPath.Text
		output := unpackOutputPath.Text
		pass := unpackPassword.Text
		if input == "" || output == "" || pass == "" {
			dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			return
		}
		if info, err := os.Stat(output); err == nil && !info.IsDir() {
			dialog.ShowInformation("ziplock", "Unpack ต้องเลือกโฟลเดอร์ปลายทาง", w)
			return
		}
		appendStatus("กำลัง unpack...")
		setBusy(true)
		go func() {
			err := unpack(input, output, pass)
			setBusy(false)
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
	unpackBtn.Importance = widget.HighImportance

	// ---------- layout helpers ----------
	field := func(label string, entry *widget.Entry, browse *widget.Button) *fyne.Container {
		return container.NewVBox(
			widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewBorder(nil, nil, nil, browse, entry),
		)
	}
	fieldNoBtn := func(label string, entry *widget.Entry) *fyne.Container {
		return container.NewVBox(
			widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			entry,
		)
	}

	packCard := widget.NewCard("📦 Pack Archive", "บีบอัดและเข้ารหัสไฟล์หรือโฟลเดอร์ที่เลือก",
		container.NewVBox(
			field("ต้นทาง", packInputPath, browsePackInput),
			field("โฟลเดอร์ปลายทาง", packOutputDir, browsePackOutputDir),
			fieldNoBtn("ชื่อไฟล์เอาต์พุต", packOutputName),
			widget.NewLabelWithStyle("รหัสผ่าน", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			packPassword,
			widget.NewSeparator(),
			container.NewCenter(packBtn),
		),
	)

	unpackCard := widget.NewCard("📂 Unpack Archive", "ถอดรหัสและแตกไฟล์ .ziplock",
		container.NewVBox(
			field("ไฟล์ .ziplock", unpackInputPath, browseUnpackInput),
			field("ปลายทาง", unpackOutputPath, browseUnpackOutput),
			widget.NewLabelWithStyle("รหัสผ่าน", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			unpackPassword,
			widget.NewSeparator(),
			container.NewCenter(unpackBtn),
		),
	)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Pack", theme.UploadIcon(), container.NewPadded(packCard)),
		container.NewTabItemWithIcon("Unpack", theme.DownloadIcon(), container.NewPadded(unpackCard)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	statusCard := widget.NewCard("สถานะการทำงาน", "", container.NewVBox(progress, status))

	title := widget.NewRichTextFromMarkdown("## 🔒 ziplock — โปรแกรมบีบอัดและเข้ารหัสไฟล์")

	split := container.NewVSplit(tabs, statusCard)
	split.SetOffset(0.72) // ให้พื้นที่ tabs เยอะกว่า status

	content := container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		nil, nil, nil,
		split,
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}
