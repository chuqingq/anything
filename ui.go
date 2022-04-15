package main

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// go build -ldflags "-H windowsgui -s -w"
func main() {
	a := app.New()
	w := a.NewWindow("Anything")
	// entry
	entry := widget.NewEntry()
	entry.PlaceHolder = "Search file here"
	entry.OnChanged = func(s string) {
		log.Printf("OnChanged: %v", s)
	}
	entry.OnSubmitted = func(s string) {
		log.Printf("OnSubmitted: %v", s)
	}
	// open button
	open := widget.NewButton("Open", func() {
		log.Printf("open")
	})
	// openFolder button
	openFolder := widget.NewButton("Open Folder", func() {
		log.Printf("open folder")
	})
	// table
	t := widget.NewTable(
		func() (int, int) { return 10, 4 },
		func() fyne.CanvasObject {
			return widget.NewLabel("Cell 000, 000")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			switch id.Col {
			case 0:
				label.SetText(fmt.Sprintf("%d", id.Row+1))
			case 1:
				label.SetText("A longer cell")
			default:
				label.SetText(fmt.Sprintf("Cell %d, %d", id.Row+1, id.Col+1))
			}
		})
	// t.SetColumnWidth(0, 34)
	// t.SetColumnWidth(1, 102)
	// content
	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, container.NewHBox(open, openFolder), entry),
		nil,
		nil,
		nil,
		t,
	))

	w.Canvas().Focus(entry)
	w.Resize(fyne.NewSize(800, 600))
	w.ShowAndRun()
}
