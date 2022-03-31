package main

import (
	"io/fs"
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
func TestFile(t *testing.T) {
	cwd, _ := os.Getwd()
	testfile1 := "testfile1"
	os.Remove(testfile1)
	defer os.Remove(testfile1)
	testfile1abs := filepath.Join(cwd, testfile1)
	log.Printf("testfile1abs: %v", testfile1abs)
	// new
	w := NewWatcher([]string{cwd})
	defer w.Close()
	ch := w.Search(testfile1)
	// 1 文件未创建
	succ := waitFor(w, ch, testfile1abs, false)
	if !succ {
		log.Fatalf("search error 1")
	}
	// 2 创建文件
	f, err := os.Create(testfile1)
	if err != nil {
		t.Fatalf("create file error: %v", err)
	}
	f.Close()
	succ = waitFor(w, ch, testfile1abs, true)
	if !succ {
		log.Fatalf("search error 2")
	}
	// 3 删除文件
	os.Remove(testfile1)
	succ = waitFor(w, ch, filepath.Join(cwd, testfile1), false)
	if !succ {
		log.Fatalf("search error 3")
	}
}

// 创建目录、删除目录
func TestDir(t *testing.T) {
	cwd, _ := os.Getwd()
	w := NewWatcher([]string{cwd})
	defer w.Close()
	//
	testdir1 := "testdir1"
	testdir1abs := filepath.Join(cwd, testdir1)
	// 1 dir
	os.Mkdir(testdir1, 0644)
	time.Sleep(100 * time.Millisecond)
	_, _, found := w.findFiles(testdir1abs)
	if !found {
		t.Fatalf("file not found1")
	}
	// remove
	os.Remove(testdir1)
	time.Sleep(100 * time.Millisecond)
	_, _, found = w.findFiles(testdir1abs)
	if found {
		t.Fatalf("file found1")
	}
}

// TODO
func TestFindFiles(t *testing.T) {
}

func TestGenerate(t *testing.T) {
	dirs := []string{"/usr/lib/go"} // "/mnt/d/temp/projects/anything" "/usr/lib/go"
	w := &Watcher{
		dirs: dirs,
	}
	w.generate2()
}

// TODO benchmark1：扫描go1.18的周期和最终内存大小
func BenchmarkGenerate(b *testing.B) {
	dirs := []string{"/usr/lib/go"} // "/mnt/d/temp/projects/anything"
	for n := 0; n < b.N; n++ {
		w := &Watcher{
			dirs: dirs,
		}
		w.generate2()
	}
}

func generate2() {
	const dir = "/usr/lib/go"

	// files := make([]fs.FileInfo, 0)
	// filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
	// 	info, err := d.Info()
	// 	if err != nil {
	// 		log.Printf("info error: %v", err)
	// 		return nil
	// 	}
	// 	files = append(files, info)
	// 	return nil
	// })
	// 24.3

	// files := make([]string, 0)
	// filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
	// 	files = append(files, path)
	// 	return nil
	// })
	// 11.2us

	type mystat struct {
		path string
		name string
		d    fs.DirEntry
	}
	files := make([]mystat, 0)
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		files = append(files, mystat{path: path, name: d.Name(), d: d})
		return nil
	})
	// 12.3us

	// files := make([]fs.FileInfo, 0)
	// filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
	// 	files = append(files, info)
	// 	return nil
	// })
	// 22.0us

	// log.Printf("%v", len(files))
}

func TestGenerate2(t *testing.T) {
	generate2()
}

func BenchmarkGenerate2(b *testing.B) {
	for n := 0; n < b.N; n++ {
		generate2()
	}
}

// TODO benchmark2：在go1.18中匹配"go go"的周期和数量
func BenchmarkSearch(b *testing.B) {
}

// TODO benchmark3：move一个文件夹，重建索引的周期、最终内存大小（不增长）
func BenchmarkMoveDir(b *testing.B) {
}

// 在timeout时间范围内，等待watcher根据file输出文件列表，然后确认findFiles中有path绝对路径
func waitFor(w *Watcher, ch chan *[]*File, path string, expectExist bool) bool {
	timer := time.After(1000 * time.Millisecond)
	for {
		select {
		case <-ch:
			_, _, found := w.findFiles(path)
			log.Printf("waitFor found: %v", found)
			if found == expectExist {
				return true
			}
		case <-timer:
			return false
		}
	}
}
