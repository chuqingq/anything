package main

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// go build -ldflags "-H windowsgui -s -w"

// TODO
// - table各列的宽度能直接拖拽
// - open和openfolder的实现
// - 按键自动focus到entry？
// - 自动最大化

func main() {
	// watcher
	dirs := []string{"C:\\Program Files\\Go"} //
	watcher := &Watcher{
		dirs: dirs,
	}
	watcher.generate()

	// data
	req := ""
	var files []File

	// font env
	os.Setenv("FYNE_FONT", "./msyh.ttc")
	// ui
	a := app.New()
	w := a.NewWindow("Anything")
	// entry
	entry := widget.NewEntry()
	entry.PlaceHolder = "Search file here"

	// table
	t := widget.NewTable(
		func() (int, int) { return 20, 4 },
		func() fyne.CanvasObject {
			return widget.NewLabel("      ")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			dir := ""
			file := ""
			if id.Row < len(files) {
				dir, file = filepath.Split(files[id.Row].Path)
			}
			// set content
			label := cell.(*widget.Label)
			switch id.Col {
			case 0:
				// label.SetText(fmt.Sprintf("%d", id.Row+1))
				label.SetText(file)
			case 1:
				label.SetText(dir)
			default:
				// label.SetText(fmt.Sprintf("Cell %d, %d", id.Row+1, id.Col+1))
				label.SetText("")
			}
		})
	// entry
	entry.OnChanged = func(s string) {
		req = s
		// TODO 不能每次都刷新
		files = watcher.Search(req)
		log.Printf("OnChanged: %v: %v", s, len(files))
		t.Refresh()
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
	t.SetColumnWidth(0, 200)
	t.SetColumnWidth(1, 1000)
	// content
	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, container.NewHBox(open, openFolder), entry),
		nil,
		nil,
		nil,
		t,
	))

	w.Resize(fyne.NewSize(800, 600))
	// w.SetFullScreen(true)
	w.Canvas().Focus(entry)
	w.ShowAndRun()
}
