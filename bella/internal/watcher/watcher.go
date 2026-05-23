package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bella/internal/ignore"
	"bella/internal/logger"
)

type Callback func() error

type FolderWatcher struct {
	Name     string
	Folder   string
	Delay    time.Duration
	Logger   *logger.Logger
	Callback Callback
	done     chan struct{}
	last     map[string]fileState
}

type fileState struct {
	ModTime int64
	Size    int64
	IsDir   bool
}

func New(name, folder string, delay time.Duration, cb Callback, log *logger.Logger) *FolderWatcher {
	return &FolderWatcher{Name: name, Folder: folder, Delay: delay, Callback: cb, Logger: log, done: make(chan struct{}), last: map[string]fileState{}}
}

func (fw *FolderWatcher) Start() error {
	if _, err := os.Stat(fw.Folder); err != nil {
		return err
	}
	snap, err := fw.snapshot()
	if err != nil {
		return err
	}
	fw.last = snap
	go fw.loop()
	fw.Logger.Success("WATCHER", fw.Name+" monitorando: "+fw.Folder)
	return nil
}

func (fw *FolderWatcher) Close() error {
	select {
	case <-fw.done:
	default:
		close(fw.done)
	}
	return nil
}

func (fw *FolderWatcher) loop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	changes := make(chan string, 100)
	go fw.debounce(changes)
	for {
		select {
		case <-fw.done:
			return
		case <-ticker.C:
			fw.check(changes)
		}
	}
}

// check compara o snapshot anterior com o atual.
// Assim a B.E.L.L.A. detecta criação, alteração e remoção sem depender de biblioteca externa.
func (fw *FolderWatcher) check(changes chan<- string) {
	current, err := fw.snapshot()
	if err != nil {
		fw.Logger.Error("WATCHER", fw.Name+" erro ao ler snapshot: "+err.Error())
		return
	}
	for path, now := range current {
		old, exists := fw.last[path]
		if !exists {
			fw.emit("criado", path, changes)
			continue
		}
		if old.ModTime != now.ModTime || old.Size != now.Size || old.IsDir != now.IsDir {
			fw.emit("alterado", path, changes)
		}
	}
	for path := range fw.last {
		if _, exists := current[path]; !exists {
			fw.emit("removido", path, changes)
		}
	}
	fw.last = current
}

func (fw *FolderWatcher) emit(kind, path string, changes chan<- string) {
	rel := fw.relative(path)
	fw.Logger.Info("WATCHER", fw.Name+" "+kind+": "+rel)
	fmt.Println(fw.Name, kind+":", rel)
	fmt.Printf("Sincronização em %d segundos...\n", int(fw.Delay.Seconds()))
	select {
	case changes <- path:
	default:
		fw.Logger.Error("WATCHER", "Fila cheia em "+fw.Name)
	}
}

func (fw *FolderWatcher) debounce(changes <-chan string) {
	var timer *time.Timer
	var timerChannel <-chan time.Time
	for {
		select {
		case <-fw.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case <-changes:
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			timer = time.NewTimer(fw.Delay)
			timerChannel = timer.C
		case <-timerChannel:
			timer = nil
			timerChannel = nil
			fmt.Println("Alterações estabilizadas em", fw.Name+". Sincronizando...")
			if err := fw.Callback(); err != nil {
				fmt.Println("Erro na sincronização", fw.Name+":", err)
				fw.Logger.Error("WATCHER", "Erro em "+fw.Name+": "+err.Error())
			}
		}
	}
}

func (fw *FolderWatcher) snapshot() (map[string]fileState, error) {
	result := map[string]fileState{}
	err := filepath.WalkDir(fw.Folder, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != fw.Folder && ignore.ShouldIgnorePath(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result[path] = fileState{ModTime: info.ModTime().UnixNano(), Size: info.Size(), IsDir: entry.IsDir()}
		return nil
	})
	return result, err
}

func (fw *FolderWatcher) relative(path string) string {
	rel, err := filepath.Rel(fw.Folder, path)
	if err != nil {
		return path
	}
	return rel
}
