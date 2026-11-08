package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.NewWithID("com.zrygan.mediauploadproducer")
	w := a.NewWindow("[Producer] Video Upload")

	label := widget.NewLabel("Upload a video below: ")

	videoLabel := widget.NewLabel("No video selected")

	selectButton := widget.NewButton("Select a video file", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			videoLabel.SetText("Selected video: " + reader.URI().Path())
		}, w)

		fileDialog.SetFilter(storage.NewExtensionFileFilter(
			[]string{".mp4", ".mkv", ".mov", ".avi", ".webm"},
		))
		fileDialog.Show()
	})

	removeButton := widget.NewButton("Remove selected file", func() {
		videoLabel.SetText("No video selected")
	})

	uploadLabel := widget.NewLabel("No video selected")

	uploadButton := widget.NewButton("Upload video to the server", func() {
		uploadLabel.SetText("Uploaded successfully!")
	})

	buttonRow := container.NewHBox(selectButton, removeButton)
	buttonRow.Resize(fyne.NewSize(100, 400))

	w.Resize(fyne.NewSize(500, 400))
	w.SetContent(container.NewVBox(label, buttonRow, videoLabel, uploadButton, uploadLabel))
	w.ShowAndRun()
}
