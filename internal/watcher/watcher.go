package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bella/internal/logger"

	"github.com/fsnotify/fsnotify"
)

type StableAction func() error

type FolderWatcher struct {
	Name   string
	Folder string
	Delay  time.Duration

	Logger   *logger.Logger
	OnStable StableAction

	watcher *fsnotify.Watcher
	done    chan struct{}
}

func New(name string, folder string, delay time.Duration, log *logger.Logger, onStable StableAction) *FolderWatcher {
	return &FolderWatcher{
		Name:     name,
		Folder:   folder,
		Delay:    delay,
		Logger:   log,
		OnStable: onStable,
		done:     make(chan struct{}),
	}
}

func (fw *FolderWatcher) Start() error {
	fileWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	fw.watcher = fileWatcher

	if err := fw.addFolderRecursive(fw.Folder); err != nil {
		_ = fw.watcher.Close()
		return err
	}

	go fw.listen()

	fw.Logger.Success("WATCHER_"+fw.Name, "Monitoramento iniciado em: "+fw.Folder)

	return nil
}

func (fw *FolderWatcher) Close() error {
	select {
	case <-fw.done:
	default:
		close(fw.done)
	}

	if fw.watcher != nil {
		return fw.watcher.Close()
	}

	return nil
}

func (fw *FolderWatcher) listen() {
	changes := make(chan string, 100)

	go fw.debounce(changes)

	for {
		select {
		case <-fw.done:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			fw.handleEvent(event, changes)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}

			fw.Logger.Error("WATCHER_"+fw.Name, "Erro no watcher: "+err.Error())
			fmt.Println("Erro no monitoramento da pasta", fw.Name+":", err)
		}
	}
}

func (fw *FolderWatcher) handleEvent(event fsnotify.Event, changes chan<- string) {
	if shouldIgnorePath(event.Name) {
		return
	}

	if event.Op&fsnotify.Create == fsnotify.Create {
		fw.tryWatchNewDirectory(event.Name)
	}

	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	relativePath := fw.relativePath(event.Name)

	message := fmt.Sprintf("[%s] Alteração detectada: %s", fw.Name, relativePath)
	fw.Logger.Info("WATCHER_"+fw.Name, message)

	fmt.Println()
	fmt.Println(message)
	fmt.Printf("[%s] Sincronização automática em %d segundos...\n", fw.Name, int(fw.Delay.Seconds()))
	fmt.Print("> ")

	select {
	case changes <- event.Name:
	default:
		fw.Logger.Error("WATCHER_"+fw.Name, "Fila de alterações cheia. Evento ignorado: "+relativePath)
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

			fmt.Println()
			fmt.Println("[" + fw.Name + "] Alterações estabilizadas. Sincronizando...")
			fw.Logger.Info("WATCHER_"+fw.Name, "Debounce finalizado. Iniciando sincronização automática.")

			if fw.OnStable == nil {
				continue
			}

			if err := fw.OnStable(); err != nil {
				fmt.Println("["+fw.Name+"] Erro na sincronização automática:", err)
				fw.Logger.Error("WATCHER_"+fw.Name, "Erro na sincronização automática: "+err.Error())
			}

			fmt.Print("> ")
		}
	}
}

func (fw *FolderWatcher) addFolderRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() {
			return nil
		}

		if shouldIgnorePath(path) && path != root {
			return filepath.SkipDir
		}

		if err := fw.watcher.Add(path); err != nil {
			return err
		}

		return nil
	})
}

func (fw *FolderWatcher) tryWatchNewDirectory(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if !info.IsDir() {
		return
	}

	if shouldIgnorePath(path) {
		return
	}

	if err := fw.addFolderRecursive(path); err != nil {
		fw.Logger.Error("WATCHER_"+fw.Name, "Erro ao monitorar nova pasta: "+err.Error())
		return
	}

	fw.Logger.Info("WATCHER_"+fw.Name, "Nova pasta monitorada: "+path)
}

func (fw *FolderWatcher) relativePath(path string) string {
	relativePath, err := filepath.Rel(fw.Folder, path)
	if err != nil {
		return path
	}

	return relativePath
}

func shouldIgnorePath(path string) bool {
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")

	ignoredFolders := map[string]bool{
		".git":         true,
		".bella":       true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
	}

	for _, part := range parts {
		if ignoredFolders[part] {
			return true
		}
	}

	ignoredExtensions := []string{
		".tmp",
		".log",
		".cache",
		".zip",
		".rar",
		".iso",
		".mp4",
	}

	lowerPath := strings.ToLower(normalized)

	for _, ext := range ignoredExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}

	return false
}
