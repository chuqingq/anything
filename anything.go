package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjeczalik/notify"
)

// TODO request：db更新后，检查结果是否有更新，如果有，更新结果
// TODO 监视文件：
// TODO close

type Watcher struct {
	dirs       []string              // 需要看护的多个路径。绝对路径，目录
	files      []*File               // 文件树索引
	searchChan chan *Search          // 查询请求通过此channel发送给watcher
	searches   []*Search             // 内部维护的查询请求
	notifyChan chan notify.EventInfo // notify channel
}

type File struct {
	Name       string
	Dir        string // 所在路径
	NameLower  string // 全小写，用于大小写不敏感的搜索
	Type       byte   // TODO
	Size       int64
	ModifyTime time.Time
	Files      []*File // 目录的子项
}

const (
	FileTypeDir  byte = iota
	FileTypeFile      // 普通文件
	// TODO 快捷方式等
)

type Search struct {
	pattern    string
	patterns   []string
	output     []*File
	outputChan chan *[]*File
}

func NewWatcher(dirs []string) *Watcher {
	w := &Watcher{
		dirs:       dirs,
		searches:   make([]*Search, 0),
		searchChan: make(chan *Search, 1),
	}
	w.watch()
	return w
}

func (w *Watcher) Close() {
	w.searchChan <- nil // 停止内部loop。 TODO 如果更可靠，还需要waitgroup等待loop退出后再清理
	// clear all searches
	for _, s := range w.searches {
		close(s.outputChan)
	}
	os.Remove(dbFile)
}

// Watch 监控文件变化
func (w *Watcher) watch() {
	// log.Printf("files: %#v, len: %v", w.files, len(w.files))
	c := make(chan notify.EventInfo, 16)
	for _, d := range w.dirs {
		abspath := filepath.Join(d, "...") // , "..."
		// TODO linux/inotify  不支持递归watch
		// log.Printf("watch path: %v", abspath)
		err := notify.Watch(abspath, c, notify.Create|notify.Remove|notify.Rename)
		if err != nil {
			log.Fatalf("ERROR watch() error: %v", err)
		}
	}
	go w.loop(c)
}

func (w *Watcher) loop(c chan notify.EventInfo) {
	// 生成文件树索引
	// w.generate()
	// 开始loop
	defer notify.Stop(c)
	// 处理fsevent
	change := false
	for {
		select {
		// fsevent
		case event := <-c:
			log.Printf("select event: %v", event)
			w.handleNotifyEvent(event.Path(), event.Event())
			change = true
			// 如果event对search有影响，则调用handleSearch生成新的output
			for _, s := range w.searches {
				if match(s.patterns, filepath.Base(event.Path())) {
					w.handleSearch(s)
				}
			}
		// 处理新请求或退出
		case search := <-w.searchChan:
			if search == nil {
				return
			}
			log.Printf("select new search: %v", search)
			w.handleSearch(search)
		// 避免连续收到fsevent，每次都写入db文件，定个超时时间
		case <-time.After(5 * time.Second):
			log.Printf("select timeout")
			if change {
				writeDB(w.files)
				change = false
			}
		}
	}
}

func (w *Watcher) handleNotifyEvent(path string, event notify.Event) {
	// TODO 暂时忽略event，刷新path
	files, index, found := w.findFiles(path)
	if files == nil {
		log.Printf("ERROR findFiles(%v) error, unexpected", path)
		return
	}
	fileInfo, err := os.Lstat(path)
	// 如果不存在，应该删除
	if os.IsNotExist(err) {
		// TODO files中删除index
		if index < 0 || index >= len(*files) {
			log.Printf("ERROR index invalid: %v, len: %v, 删除的文件，但是db中没有，unexpected", len(*files), index)
			return
		}
		*files = append((*files)[0:index], (*files)[index+1:]...)
		return
	}
	if err != nil {
		log.Printf("ERROR lstate(%v) error: %v", path, err)
		return
	}
	// 如果找到了，就更新该File
	if found {
		log.Printf("ERROR 找到了File. unexpected")
		update("", (*files)[index], fileInfo)
	} else {
		// 如果没找到，需要添加
		dir, file := filepath.Split(path)
		f := &File{
			Name: file,
			Dir:  dir,
		}
		// *files = append((*files)[0:index], f, (*files)[index:]...)
		*files = append(*files, nil)               // 切片扩展1个空间
		copy((*files)[index+1:], (*files)[index:]) // a[i:]向后移动1个位置
		(*files)[index] = f                        // 设置新添加的元素
		update("", f, fileInfo)
	}
}

// result1: path应当所属的files
// result2: path应该当在files中的位置
// resutl3: result2是否就是path对应的file。如果为false，可能是需要在result2处新增file
func (w *Watcher) findFiles(path string) (*[]*File, int, bool) {
	// TODO
	for _, f := range w.files {
		fabs := filepath.Join(f.Dir, f.Name)
		if strings.HasPrefix(path, fabs) {
			suffix := strings.TrimPrefix(path, fabs)
			dirs := strings.Split(strings.Trim(suffix, string(os.PathSeparator)), string(os.PathSeparator))
			r, ind, found := findFiles(f, dirs)
			return r, ind, found
		}
	}
	return nil, 0, false
}

func findFiles(f *File, dirs []string) (*[]*File, int, bool) {
	// log.Printf("findFiles: f:%v, dirs: %v", f.Name, dirs)
	if len(dirs) == 0 {
		log.Printf("ERROR findFiles len(dir) == 0, unexpected")
		return nil, 0, false
	}
	if len(dirs) == 1 {
		for i, c := range f.Files {
			r := strings.Compare(dirs[0], c.Name)
			if r == 0 {
				return &f.Files, i, true
			} else if r < 0 {
				return &f.Files, i, false
			}
		}
		return &f.Files, len(f.Files), false
	}
	//
	for _, c := range f.Files {
		if f.Name == c.Name {
			fs, index, found := findFiles(c, dirs[1:])
			return fs, index, found
		}
	}
	log.Printf("ERROR findFiles dirs[0](%v) not found, unexpected", dirs[0])
	return nil, 0, false
}

const INDENT = "  "

func (w *Watcher) generate() {
	w.files = make([]*File, len(w.dirs), len(w.dirs))
	for i := 0; i < len(w.dirs); i++ {
		dir, base := filepath.Split(w.dirs[i])
		// log.Printf("load: %v, %v", dir, base)
		w.files[i] = &File{
			// NameAbs:   db.dirs[i],
			Name: base,
			// NameLower: strings.ToLower(base),
			Dir: dir,
		}
	}

	for _, f := range w.files {
		update("", f, nil)
	}
	// log.Printf("generate success; enter handleSearch")
	// 给search返回结果
	for _, s := range w.searches {
		w.handleSearch(s)
	}
	// debug
	writeDB(w.files)
}

// 全量刷新
func update(indent string, f *File, fileInfo fs.FileInfo) {
	// log.Printf("#%v%v", indent, fileInfo.Name())
	if fileInfo == nil {
		var err error
		fileInfo, err = os.Lstat(filepath.Join(f.Dir, f.Name)) // f.NameAbs
		if err != nil {
			log.Printf("ERROR lstat error: %v", err)
			return
		}
	}
	// 1.更新File本身
	// f.NameAbs在上级更新
	f.Name = fileInfo.Name()
	f.NameLower = strings.ToLower(f.Name)
	// TODO f.Type
	f.Size = fileInfo.Size()
	// f.Dir在上级更新
	f.ModifyTime = fileInfo.ModTime()
	// 2.更新f.Files
	if !fileInfo.IsDir() {
		f.Files = nil
		// log.Printf("file refreshed")
		return
	}
	// log.Printf("###isdir: %v", f.NameAbs)
	dirEntrys, err := os.ReadDir(filepath.Join(f.Dir, f.Name)) // f.NameAbs
	if err != nil {
		log.Printf("ERROR readdir error: %v", err)
		return
	}
	f.Files = make([]*File, 0)
	for _, d := range dirEntrys {
		// log.Printf("%v ==> %v", f.NameAbs, d.Name())
		fi, err := d.Info()
		if err != nil {
			log.Printf("ERROR: d.Info() error: %v", err)
			continue // TODO
		}
		newf := &File{
			// NameAbs: filepath.Join(f.Dir, f.Name, d.Name()),
			Name: d.Name(),
			Dir:  filepath.Join(f.Dir, f.Name),
		}
		f.Files = append(f.Files, newf)
		update(indent+INDENT, newf, fi)
	}
}

func (w *Watcher) Search(pattern string) chan *[]*File {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	search := &Search{
		pattern:    pattern,
		patterns:   strings.Fields(pattern),
		output:     make([]*File, 0),
		outputChan: make(chan *[]*File, 1),
	}
	w.searchChan <- search
	return search.outputChan
}

func (w *Watcher) handleSearch(s *Search) {
	log.Printf("handleSearch(%v)", s.pattern)
	// 先把search请求放在内部
	w.searches = append(w.searches, s)
	// 执行search请求
	search(s.patterns, w.files, &s.output) // TODO 未优化
	// 发送search响应。不能阻塞
	select {
	case s.outputChan <- &s.output:
	default:
		log.Printf("ERROR output error")
	}
	log.Printf("handleSearch: %v", len(s.output))
	for _, f := range s.output {
		log.Printf("\t%v", filepath.Join(f.Dir, f.Name))
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

func search(patterns []string, input []*File, output *[]*File) {
	for _, f := range input {
		contains := match(patterns, f.NameLower)
		if contains {
			*output = append(*output, f)
		}
		// child
		if len(f.Files) != 0 {
			search(patterns, f.Files, output)
		}
	}
}

func (w *Watcher) StopSearch(outputChan chan *[]*File) {
	index := -1
	for i, s := range w.searches {
		if s.outputChan == outputChan {
			index = i
			break
		}
	}
	if index > 0 && index < len(w.searches) {
		w.searches = append(w.searches[:index], w.searches[index+1:]...)
		close(outputChan)
	} else {
		log.Printf("ERROR stopsearch outputChan not found, unexpected")
	}
}

// database read and write
const dbFile = "./anything.db"

func readDB(files *[]*File) error {
	f, err := os.OpenFile(dbFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	return decoder.Decode(files)
}

func writeDB(files []*File) error {
	f, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	return encoder.Encode(files)
}
