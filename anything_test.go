package main

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// const dir = "/usr"
// const dir = "/mnt/d/temp/projects/anything"
// const dir = "D:\\temp\\projects\\anything"

// 创建文件、删除文件
func TestSearch(t *testing.T) {
	cwd, _ := os.Getwd()
	testfile1 := "testfile1"
	testfile1abs := filepath.Join(cwd, testfile1)
	log.Printf("testfile1abs: %v", testfile1abs)
	os.Remove(testfile1abs)
	defer os.Remove(testfile1abs)
	// new
	w := NewWatcher([]string{cwd})
	defer w.Close()
	// 1 文件未创建
	res := w.Search(testfile1)
	if len(res) != 0 {
		log.Fatalf("search error 1")
	}
	// 2 创建文件
	f, err := os.Create(testfile1abs)
	if err != nil {
		t.Fatalf("create file error: %v", err)
	}
	f.Close()
	time.Sleep(200 * time.Millisecond)
	res = w.Search(testfile1)
	if len(res) == 0 {
		log.Fatalf("search error 2")
	}
	// 3 删除文件
	os.Remove(testfile1abs)
	time.Sleep(200 * time.Millisecond)
	res = w.Search(testfile1)
	if len(res) != 0 {
		log.Fatalf("search error 3")
	}
}

func TestWatch(t *testing.T) {
	cwd, _ := os.Getwd()
	testfile1 := "testfile1"
	testfile1abs := filepath.Join(cwd, testfile1)
	log.Printf("testfile1abs: %v", testfile1abs)
	os.Remove(testfile1abs)
	defer os.Remove(testfile1abs)
	// new
	w := NewWatcher([]string{cwd})
	defer w.Close()
	ch := w.Watch(testfile1)
	// 1 文件未创建
	succ := waitFor(w, ch, testfile1abs, false)
	if !succ {
		log.Fatalf("search error 1")
	}
	// 2 创建文件
	f, err := os.Create(testfile1abs)
	if err != nil {
		t.Fatalf("create file error: %v", err)
	}
	f.Close()
	succ = waitFor(w, ch, testfile1abs, true)
	if !succ {
		log.Fatalf("search error 2")
	}
	// 3 删除文件
	os.Remove(testfile1abs)
	succ = waitFor(w, ch, testfile1abs, false)
	if !succ {
		// log.Printf("search error 3")
		log.Fatalf("search error 3")
	}
}

// 创建目录、删除目录
// func TestDir(t *testing.T) {
// 	cwd, _ := os.Getwd()
// 	w := NewWatcher([]string{cwd})
// 	defer w.Close()
// 	//
// 	testdir1 := "testdir1"
// 	testdir1abs := filepath.Join(cwd, testdir1)
// 	// 1 dir
// 	os.Mkdir(testdir1, 0644)
// 	time.Sleep(100 * time.Millisecond)
// 	_, _, found := w.findFiles(testdir1abs)
// 	if !found {
// 		t.Fatalf("file not found1")
// 	}
// 	// remove
// 	os.Remove(testdir1)
// 	time.Sleep(100 * time.Millisecond)
// 	_, _, found = w.findFiles(testdir1abs)
// 	if found {
// 		t.Fatalf("file found1")
// 	}
// }

// TODO
// func TestFindFiles(t *testing.T) {
// }

// func TestGenerate(t *testing.T) {
// 	// dirs := []string{"/usr/lib/go"} // "/mnt/d/temp/projects/anything" "/usr/lib/go"
// 	// w := &Watcher{
// 	// 	dirs: dirs,
// 	// }
// 	// w.
// 	generate2()
// }

// benchmark1：扫描go1.18的周期和最终内存大小
func BenchmarkGenerate(b *testing.B) {
	dirs := []string{"/usr/lib/go"} // "/mnt/d/temp/projects/anything"
	w := &Watcher{
		dirs: dirs,
	}
	for n := 0; n < b.N; n++ {
		// w.files = nil
		w.generate()
	}
	// 13.2us
	// 11.8us 复用[]File
}

// func TestOrder(t *testing.T) {
// 	dirs := []string{"/mnt/d/temp/projects/anything2"} //
// 	w := &Watcher{
// 		dirs: dirs,
// 	}
// 	w.generate()
// 	// 不是按顺序的
// 	// last := ""
// 	for _, f := range w.files {
// 		log.Printf("%v", f.Path)
// 		// if strings.Compare(last, f.Path) >= 0 {
// 		// 	log.Printf("")
// 		// }
// 		// last = f.Path
// 	}
// }

// func BenchmarkOrder(b *testing.B) {
// 	dirs := []string{"/mnt/d/temp/projects/anything2"} //
// 	w := &Watcher{
// 		dirs: dirs,
// 	}
// 	w.generate()
// 	// 不是按顺序的。就按照walk的顺序即可！！！
// 	// last := ""
// 	for _, f := range w.files {
// 		log.Printf("%v", f.Path)
// 		// if strings.Compare(last, f.Path) >= 0 {
// 		// 	log.Printf("")
// 		// }
// 		// last = f.Path
// 	}
// 	// for n := 0; n < b.N; n++ {
// 	// 	w.files = nil // not slice

// 	// 	// log.Printf("%v", len(w.files))
// 	// }
// }

// benchmark2：在go1.18中匹配"go file"的周期和数量
func BenchmarkSearchWithoutCache(b *testing.B) {
	dirs := []string{"/usr/lib/go"} //
	w := &Watcher{
		dirs: dirs,
	}
	w.generate()
	for n := 0; n < b.N; n++ {
		output := w.Search("go file")
		// log.Printf("%v", len(output))
		if len(output) != 88 {
			b.FailNow()
		}
	}
	// 0.41us
}

// TODO 带缓存的Search
func BenchmarkSearchWithCache(b *testing.B) {
	// TODO
}

// func generate2() {
// 	const dir = "/usr/lib/go"
// 	// const dir = "/mnt/d/temp/projects/anything2"

// 	// files := make([]fs.FileInfo, 0)
// 	// filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
// 	// 	info, err := d.Info()
// 	// 	if err != nil {
// 	// 		log.Printf("info error: %v", err)
// 	// 		return nil
// 	// 	}
// 	// 	files = append(files, info)
// 	// 	return nil
// 	// })
// 	// 24.3

// 	// files := make([]string, 0)
// 	// filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
// 	// 	files = append(files, path)
// 	// 	return nil
// 	// })
// 	// 11.2us

// 	type mystat struct {
// 		Path string
// 		name string
// 		d    fs.DirEntry
// 	}
// 	files := make([]mystat, 0)
// 	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
// 		files = append(files, mystat{Path: path, name: d.Name(), d: d})
// 		return nil
// 	})
// 	// 12.3us
// 	// writeDB2(files)

// 	// files := make([]fs.FileInfo, 0)
// 	// filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
// 	// 	files = append(files, info)
// 	// 	return nil
// 	// })
// 	// 22.0us

// 	// log.Printf("%v", len(files))
// }

// func writeDB2(v interface{}) error {
// 	const dbFile = "anything.db"
// 	f, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE, 0644)
// 	if err != nil {
// 		return err
// 	}
// 	defer f.Close()
// 	encoder := json.NewEncoder(f)
// 	return encoder.Encode(v)
// }

// func TestGenerate2(t *testing.T) {
// 	generate2()
// }

// func BenchmarkGenerate2(b *testing.B) {
// 	for n := 0; n < b.N; n++ {
// 		generate2()
// 	}
// }

// TODO benchmark3：move一个文件夹，重建索引的周期、最终内存大小（不增长）
func BenchmarkMoveDir(b *testing.B) {
}

// 在timeout时间范围内，等待watcher根据file输出文件列表，然后确认findFiles中有path绝对路径
func waitFor(w *Watcher, ch chan []File, path string, expectExist bool) bool {
	timer := time.After(1000 * time.Millisecond)
	for {
		select {
		case res := <-ch:
			found := false
			for _, f := range res {
				if f.Path == path {
					found = true
					break
				}
			}
			// log.Printf("waitFor found: %v", found)
			if found == expectExist {
				return true
			} else {
				// log.Printf("files: %v", *res)
			}
		case <-timer:
			return false
		}
	}
}

// func filesSame(files1 []File, files2 []File) bool {
// 	if len(files1) != len(files2) {
// 		return false
// 	}
// 	for i, f := range files1 {
// 		if f.Path != files2[i].Path {
// 			return false
// 		}
// 	}
// 	return true
// }

// func TestSort(t *testing.T) {
// 	a := []string{"1", "3", "9", "7", "5"}
// 	sort.StringSlice(a).Sort()
// 	log.Printf("%v", a)
// 	// 2022/04/02 00:22:30 [1 3 5 7 9]
// }
