package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rjeczalik/notify"
)

// TODO request：db更新后，检查结果是否有更新，如果有，更新结果
// TODO 监视文件：
// TODO close

type Watcher struct {
	dirs       []string              // 需要看护的多个路径。绝对路径，目录
	files      []File                // 文件树索引
	watchChan  chan *Watch           // 查询请求通过此channel发送给watcher
	watches    []*Watch              // 内部维护的查询请求
	events     []notify.EventInfo    // 缓存的events
	notifyChan chan notify.EventInfo // notify channel
}

func NewWatcher(dirs []string) *Watcher {
	w := &Watcher{
		dirs: dirs,
		// files:      make([]File, 0), // 后续只append，因此这里不需要make
		watches:   make([]*Watch, 0),
		watchChan: make(chan *Watch, 1),
	}
	w.watch()
	go w.generateAndLoop()
	return w
}

func (w *Watcher) Close() {
	w.watchChan <- nil // 停止内部loop。 TODO 如果更可靠，还需要waitgroup等待loop退出后再清理
	// clear all searches
	for _, s := range w.watches {
		close(s.outputChan)
	}
	os.Remove(dbFile)
}

func (w *Watcher) Search(pattern string) []File {
	// TODO
	// 1. searchCache
	// 2. search
	patterns := parsePattern(pattern)
	// if patterns == nil {
	// 	return nil
	// }
	output := make([]File, 0)
	search(patterns, w.files, &output)
	return output
}

func (w *Watcher) Watch(pattern string) chan *[]File {
	patterns := parsePattern(pattern)
	if patterns == nil {
		return nil
	}
	search := &Watch{
		pattern:  strings.Join(patterns, " "),
		patterns: patterns,
		// output:     make([]File, 0),
		outputChan: make(chan *[]File, 1),
	}
	w.watchChan <- search
	return search.outputChan
}

// StopWatch 应该loop协程来close chan
func (w *Watcher) StopWatch(outputChan chan *[]File) {
	watch := &Watch{
		// patterns == nil 用于关闭watch
		outputChan: outputChan,
	}
	w.watchChan <- watch
}

// internal

type File struct {
	Name      string
	Path      string
	NameLower string // 全小写，用于大小写不敏感的搜索
	DirEntry  fs.DirEntry
}

type Watch struct {
	pattern    string
	patterns   []string
	output     []File
	outputChan chan *[]File
}

func parsePattern(pattern string) []string {
	patterns := strings.Fields(strings.TrimSpace(strings.ToLower(pattern)))
	// if len(patterns) == 0 {
	// 	return nil
	// }
	// TODO 还要去重
	sort.StringSlice(patterns).Sort()
	return patterns
}

// Watch 监控文件变化
func (w *Watcher) watch() {
	// log.Printf("files: %#v, len: %v", w.files, len(w.files))
	w.notifyChan = make(chan notify.EventInfo, 16)
	for _, d := range w.dirs {
		abspath := filepath.Join(d, "...") // , "..."
		// TODO linux/inotify  不支持递归watch
		// log.Printf("watch path: %v", abspath)
		err := notify.Watch(abspath, w.notifyChan, notify.Create|notify.Remove|notify.Rename)
		if err != nil {
			log.Fatalf("ERROR watch() error: %v", err)
		}
	}
}

func (w *Watcher) generateAndLoop() {
	// 生成文件树索引
	w.generate()
	w.loop()
}

// 全量生成
func (w *Watcher) generate() {
	for _, dir := range w.dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			w.files = append(w.files, File{Path: path, Name: d.Name(), DirEntry: d})
			return nil
		})
	}
}

func (w *Watcher) loop() {
	defer notify.Stop(w.notifyChan)
	for {
		select {
		// fsevent
		case event := <-w.notifyChan:
			log.Printf("select event: %v", event)
			w.events = append(w.events, event)
		// 处理新请求或退出
		case watch := <-w.watchChan:
			if watch == nil {
				return
			}
			if watch.patterns == nil {
				w.handleStopWatch(watch)
			} else {
				// 先把search请求放在内部
				w.watches = append(w.watches, watch)
				w.handleWatch(watch)
			}
		// 避免连续收到fsevent每次都全刷新
		case <-time.After(100 * time.Millisecond):
			// 如果变化，重新生成
			if len(w.events) > 0 {
				// 全量刷新
				w.files = w.files[:0]
				w.generate()
				// 受影响的watch重新search
				for _, watch := range w.watches {
					if isWatchAffectedByEvents(watch, w.events) {
						w.handleWatch(watch)
					}
				}
				w.events = w.events[:0]
			}
		}
	}
}

func isWatchAffectedByEvents(w *Watch, events []notify.EventInfo) bool {
	for _, e := range events {
		name := strings.ToLower(filepath.Base(e.Path()))
		if match(w.patterns, name) {
			return true
		}
	}
	return false
}

// func (w *Watcher) handleNotifyEvent(event notify.EventInfo) {
// 	w.events = append(w.events, event)
// 	// // TODO 暂时忽略event，刷新path
// 	// files, index, found := w.findFiles(path)
// 	// if files == nil {
// 	// 	log.Printf("ERROR findFiles(%v) error, unexpected", path)
// 	// 	return
// 	// }
// 	// _, err := os.Lstat(event.Path())
// 	// // 如果不存在，应该删除
// 	// if os.IsNotExist(err) {
// 	// 	// TODO files中删除index
// 	// 	if index < 0 || index >= len(*files) {
// 	// 		log.Printf("ERROR index invalid: %v, len: %v, 删除的文件，但是db中没有，unexpected", len(*files), index)
// 	// 		return
// 	// 	}
// 	// 	*files = append((*files)[0:index], (*files)[index+1:]...)
// 	// 	return
// 	// }
// 	// if err != nil {
// 	// 	log.Printf("ERROR lstate(%v) error: %v", path, err)
// 	// 	return
// 	// }
// 	// // 如果找到了，就更新该File
// 	// if found {
// 	// 	log.Printf("ERROR 找到了File. unexpected")
// 	// 	// update("", (*files)[index], fileInfo)
// 	// } else {
// 	// 	// 如果没找到，需要添加
// 	// 	_, file := filepath.Split(path)
// 	// 	f := &File{
// 	// 		Name: file,
// 	// 		// Dir:  dir,
// 	// 	}
// 	// 	// *files = append((*files)[0:index], f, (*files)[index:]...)
// 	// 	*files = append(*files, nil)               // 切片扩展1个空间
// 	// 	copy((*files)[index+1:], (*files)[index:]) // a[i:]向后移动1个位置
// 	// 	(*files)[index] = f                        // 设置新添加的元素
// 	// 	// update("", f, fileInfo)
// 	// }

// }

// result1: path应当所属的files
// result2: path应该当在files中的位置
// resutl3: result2是否就是path对应的file。如果为false，可能是需要在result2处新增file
// func (w *Watcher) findFiles(path string) (*[]*File, int, bool) {
// 	// TODO
// 	for _, f := range w.files {
// 		fabs := f.Path // .Join(f.Dir, f.Name)
// 		if strings.HasPrefix(path, fabs) {
// 			suffix := strings.TrimPrefix(path, fabs)
// 			dirs := strings.Split(strings.Trim(suffix, string(os.PathSeparator)), string(os.PathSeparator))
// 			r, ind, found := findFiles(f, dirs)
// 			return r, ind, found
// 		}
// 	}
// 	return nil, 0, false
// }

// func findFiles(f File, dirs []string) (*[]*File, int, bool) {
// 	// // log.Printf("findFiles: f:%v, dirs: %v", f.Name, dirs)
// 	// if len(dirs) == 0 {
// 	// 	log.Printf("ERROR findFiles len(dir) == 0, unexpected")
// 	// 	return nil, 0, false
// 	// }
// 	// if len(dirs) == 1 {
// 	// 	for i, c := range f.Files {
// 	// 		r := strings.Compare(dirs[0], c.Name)
// 	// 		if r == 0 {
// 	// 			return &f.Files, i, true
// 	// 		} else if r < 0 {
// 	// 			return &f.Files, i, false
// 	// 		}
// 	// 	}
// 	// 	return &f.Files, len(f.Files), false
// 	// }
// 	// //
// 	// for _, c := range f.Files {
// 	// 	if f.Name == c.Name {
// 	// 		fs, index, found := findFiles(c, dirs[1:])
// 	// 		return fs, index, found
// 	// 	}
// 	// }
// 	// log.Printf("ERROR findFiles dirs[0](%v) not found, unexpected", dirs[0])
// 	return nil, 0, false
// }

// const INDENT = "  "

// func (w *Watcher) generate() {
// 	w.files = make([]*File, len(w.dirs), len(w.dirs))
// 	for i := 0; i < len(w.dirs); i++ {
// 		dir, base := filepath.Split(w.dirs[i])
// 		// log.Printf("load: %v, %v", dir, base)
// 		w.files[i] = &File{
// 			// NameAbs:   db.dirs[i],
// 			Name: base,
// 			// NameLower: strings.ToLower(base),
// 			Dir: dir,
// 		}
// 	}

// 	for _, f := range w.files {
// 		update("", f, nil)
// 	}
// 	// log.Printf("generate success; enter handleSearch")
// 	// // 给search返回结果
// 	// for _, s := range w.searches {
// 	// 	w.handleSearch(s)
// 	// }
// 	// // debug
// 	// writeDB(w.files)
// }

// // 全量刷新
// func update(indent string, f *File, fileInfo fs.FileInfo) {
// 	// log.Printf("#%v%v", indent, fileInfo.Name())
// 	if fileInfo == nil {
// 		var err error
// 		fileInfo, err = os.Lstat(filepath.Join(f.Dir, f.Name)) // f.NameAbs
// 		if err != nil {
// 			log.Printf("ERROR lstat error: %v", err)
// 			return
// 		}
// 	}
// 	// 1.更新File本身
// 	// f.NameAbs在上级更新
// 	f.Name = fileInfo.Name()
// 	f.NameLower = strings.ToLower(f.Name)
// 	// TODO f.Type
// 	f.Size = fileInfo.Size()
// 	// f.Dir在上级更新
// 	f.ModifyTime = fileInfo.ModTime()
// 	// 2.更新f.Files
// 	if !fileInfo.IsDir() {
// 		f.Files = nil
// 		// log.Printf("file refreshed")
// 		return
// 	}
// 	// log.Printf("###isdir: %v", f.NameAbs)
// 	dirEntrys, err := os.ReadDir(filepath.Join(f.Dir, f.Name)) // f.NameAbs
// 	if err != nil {
// 		log.Printf("ERROR readdir error: %v", err)
// 		return
// 	}
// 	f.Files = make([]*File, 0)
// 	for _, d := range dirEntrys {
// 		// log.Printf("%v ==> %v", f.NameAbs, d.Name())
// 		fi, err := d.Info()
// 		if err != nil {
// 			log.Printf("ERROR: d.Info() error: %v", err)
// 			continue // TODO
// 		}
// 		newf := &File{
// 			// NameAbs: filepath.Join(f.Dir, f.Name, d.Name()),
// 			Name: d.Name(),
// 			Dir:  filepath.Join(f.Dir, f.Name),
// 		}
// 		f.Files = append(f.Files, newf)
// 		update(indent+INDENT, newf, fi)
// 	}
// }

func (w *Watcher) handleWatch(s *Watch) {
	log.Printf("handleSearch(%v)", s.pattern)

	// 执行search请求
	search(s.patterns, w.files, &s.output) // TODO 未优化
	// 发送search响应。不能阻塞
	select {
	case s.outputChan <- &s.output:
	default:
		log.Printf("ERROR output error")
	}
}

func search(patterns []string, input []File, output *[]File) {
	for _, f := range input {
		if len(f.NameLower) == 0 {
			f.NameLower = strings.ToLower(f.Name)
		}
		contains := match(patterns, f.NameLower)
		if contains {
			*output = append(*output, f)
		}
	}
}

func match(patterns []string, name string) bool {
	for _, p := range patterns {
		if !strings.Contains(name, p) {
			return false
		}
	}
	return true
}

func (w *Watcher) handleStopWatch(watch *Watch) {
	index := -1
	for i, s := range w.watches {
		if s.outputChan == watch.outputChan {
			index = i
			break
		}
	}
	if index == -1 {
		log.Printf("ERROR stopsearch outputChan not found, unexpected")
	} else {
		w.watches = append(w.watches[:index], w.watches[index+1:]...)
		close(watch.outputChan)
	}
}
