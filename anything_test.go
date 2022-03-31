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

// TODO benchmark1：扫描go1.18的周期和最终内存大小
func BenchmarkGenerate(b *testing.B) {
	dirs := []string{"/mnt/d/temp/projects/anything"} // "/usr/lib/go"
	for n := 0; n < b.N; n++ {
		w := &Watcher{
			dirs: dirs,
		}
		w.generate()
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
