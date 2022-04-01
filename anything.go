package main

import (
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rjeczalik/notify"
)

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
}

// Search 一次性查询
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

// Watch 观察。结果有变化时会通过channel返回
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

// StopWatch 停止观察。这里只发送停止请求，由内部协程处理。
func (w *Watcher) StopWatch(outputChan chan *[]File) {
	watch := &Watch{
		// 约定：patterns == nil 用于关闭watch
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
	pattern    string // TODO 后续用于作为缓存的key
	patterns   []string
	output     []File // TODO 可以不要
	outputChan chan *[]File
}

func parsePattern(pattern string) []string {
	// 转小写、去空白、分割
	patterns := strings.Fields(strings.TrimSpace(strings.ToLower(pattern)))
	// if len(patterns) == 0 {
	// 	return nil
	// }
	// 去重
	m := map[string]struct{}{}
	for _, p := range patterns {
		m[p] = struct{}{}
	}
	patterns = patterns[:0]
	for k := range m {
		patterns = append(patterns, k)
	}
	// 排序
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

func (w *Watcher) handleWatch(watch *Watch) {
	log.Printf("handleSearch(%v)", watch.pattern)
	// 执行search请求
	search(watch.patterns, w.files, &watch.output) // TODO 未优化
	// 发送search响应。不能阻塞
	select {
	case watch.outputChan <- &watch.output:
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
