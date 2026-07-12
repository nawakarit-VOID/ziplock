package main

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func runGUI() {
	a := app.New()
	w := a.NewWindow("ziplock")
	w.Resize(fyne.NewSize(760, 520))

	inputPath := widget.NewEntry()
	inputPath.SetPlaceHolder("เลือกไฟล์ต้นทาง")
	outputPath := widget.NewEntry()
	outputPath.SetPlaceHolder("กำหนดไฟล์ปลายทาง")
	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("ใส่รหัสผ่าน")
	status := widget.NewMultiLineEntry()
	status.SetText("พร้อมใช้งาน")
	status.Disable()

	appendStatus := func(msg string) {
		if current := status.Text; current == "" {
			status.SetText(msg)
		} else {
			status.SetText(current + "\n" + msg)
		}
	}

	browseInput := widget.NewButton("เลือกไฟล์", func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			inputPath.SetText(r.URI().Path())
			_ = r.Close()
		}, w)
	})

	browseOutput := widget.NewButton("เลือกปลายทาง", func() {
		dialog.ShowFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			outputPath.SetText(wc.URI().Path())
			_ = wc.Close()
		}, w)
	})

	packBtn := widget.NewButton("Pack", func() {
		if inputPath.Text == "" || outputPath.Text == "" || password.Text == "" {
			dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			return
		}
		appendStatus("กำลัง pack...")
		go func() {
			err := pack(inputPath.Text, outputPath.Text, password.Text)
			fyne.Do(func() {
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
		if inputPath.Text == "" || outputPath.Text == "" || password.Text == "" {
			dialog.ShowInformation("ziplock", "กรุณากรอกข้อมูลให้ครบ", w)
			return
		}
		appendStatus("กำลัง unpack...")
		go func() {
			err := unpack(inputPath.Text, outputPath.Text, password.Text)
			fyne.Do(func() {
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
