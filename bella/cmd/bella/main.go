package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bella/internal/config"
	bellagit "bella/internal/git"
	"bella/internal/ignore"
	"bella/internal/logger"
	"bella/internal/startup"
	"bella/internal/state"
	bellasync "bella/internal/sync"
	"bella/internal/usb"
	"bella/internal/watcher"
)

type App struct {
	AppDir          string
	ConfigPath      string
	Config          *config.Config
	Log             *logger.Logger
	Git             *bellagit.Git
	State           *state.Store
	Sync            *bellasync.SyncManager
	localWatcher    *watcher.FolderWatcher
	portableWatcher *watcher.FolderWatcher
	usbWatcher      *watcher.FolderWatcher
	input           chan string
	usbEvents       chan bool
	stop            chan struct{}
}

func main() {
	app, err := bootstrap()
	if err != nil {
		fmt.Println("Erro:", err)
		return
	}
	defer app.shutdown()
	app.printHeader()
	if err := app.prepare(); err != nil {
		fmt.Println("Erro ao preparar B.E.L.L.A.:", err)
		app.Log.Error("SYSTEM", err.Error())
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--sync-now" {
		app.runImmediateSync()
	}
	app.commandLoop()
}

func bootstrap() (*App, error) {
	appDir, configPath, err := resolveAppPaths()
	if err != nil {
		return nil, err
	}
	cfg, firstRun, err := config.LoadOrCreateFirstRun(configPath)
	if err != nil {
		return nil, err
	}
	if firstRun {
		return nil, fmt.Errorf("config.json não encontrado. Copie config.installed.json ou config.portable.json para config.json e ajuste os dados")
	}
	if err := cfg.ResolvePaths(appDir); err != nil {
		return nil, err
	}
	log, err := logger.New(appDir)
	if err != nil {
		return nil, err
	}
	st, err := state.New(appDir, cfg.Mode)
	if err != nil {
		return nil, err
	}
	gitClient := bellagit.New(cfg.GitPath)
	if !gitClient.IsGitInstalled() && cfg.FallbackToSystemGit && cfg.GitPath != "git" {
		log.Info("GIT", "Git configurado não encontrado. Tentando Git do sistema.")
		gitClient = bellagit.New("git")
		cfg.GitPath = "git"
	}
	syncManager := bellasync.New(cfg, gitClient, log, st)
	return &App{AppDir: appDir, ConfigPath: configPath, Config: cfg, Log: log, Git: gitClient, State: st, Sync: syncManager, input: make(chan string), usbEvents: make(chan bool, 4), stop: make(chan struct{})}, nil
}

func (a *App) prepare() error {
	a.Log.Info("SYSTEM", "B.E.L.L.A. v0.2.1 iniciada")
	if !a.Git.IsGitInstalled() {
		return fmt.Errorf("Git não encontrado. Ajuste gitPath, instale o Git ou configure PortableGit")
	}
	if a.Config.NoSaveCredentials {
		fmt.Println("Modo sem salvar credenciais ativo. Evite salvar login Git em computadores desconhecidos.")
	}
	if a.Config.IsInstalled() {
		return a.prepareInstalled()
	}
	return a.preparePortable()
}

func (a *App) prepareInstalled() error {
	if err := os.MkdirAll(a.Config.LocalFolder, os.ModePerm); err != nil {
		return err
	}
	if err := ignore.EnsureGitignore(a.Config.LocalFolder, a.Config.IgnorePatterns); err != nil {
		return err
	}
	if !bellagit.IsRepository(a.Config.LocalFolder) {
		if err := a.Git.InitRepository(a.Config.LocalFolder, a.Config.RepoURL, a.Config.Branch); err != nil {
			return err
		}
	}
	_ = a.Git.SetUpstream(a.Config.LocalFolder, a.Config.Branch)
	a.localWatcher = watcher.New("LOCAL", a.Config.LocalFolder, a.Config.AutoCommitDelay(), a.Sync.SyncLocal, a.Log)
	if err := a.localWatcher.Start(); err != nil {
		return err
	}
	go a.usbMonitor()
	return nil
}

func (a *App) preparePortable() error {
	if err := usb.EnsureRepository(a.Config.RepositoryFolder, a.Config.RepoURL, a.Config.Branch, a.Git, a.Log, "PORTABLE"); err != nil {
		return err
	}
	if err := ignore.EnsureGitignore(a.Config.RepositoryFolder, a.Config.IgnorePatterns); err != nil {
		return err
	}
	_ = a.Git.SetUpstream(a.Config.RepositoryFolder, a.Config.Branch)
	a.portableWatcher = watcher.New("PORTABLE", a.Config.RepositoryFolder, a.Config.AutoCommitDelay(), a.Sync.SyncPortable, a.Log)
	if err := a.portableWatcher.Start(); err != nil {
		return err
	}
	if a.Config.AskBeforePortableSync {
		fmt.Println("Deseja sincronizar o modo portable agora? [s/n]")
	} else if a.Config.SyncOnStart {
		_ = a.Sync.SyncPortable()
	}
	return nil
}

func (a *App) usbMonitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	last := usb.IsConnected(a.Config.USBDriveLetter)
	if last {
		a.usbEvents <- true
	}
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
			cur := usb.IsConnected(a.Config.USBDriveLetter)
			if cur != last {
				last = cur
				a.usbEvents <- cur
			}
		}
	}
}

func (a *App) handleUSBConnected() {
	fmt.Println("Pendrive detectado em", a.Config.USBDriveLetter)
	if err := usb.EnsureUSBRepository(a.Config, a.Git, a.Log); err != nil {
		fmt.Println("Erro ao preparar pendrive:", err)
		return
	}
	if a.usbWatcher != nil {
		_ = a.usbWatcher.Close()
	}
	a.usbWatcher = watcher.New("USB", a.Config.USBFolder, a.Config.AutoCommitDelay(), a.Sync.SyncUSB, a.Log)
	if err := a.usbWatcher.Start(); err != nil {
		fmt.Println("Erro ao monitorar pendrive:", err)
		return
	}
	if a.Config.AskBeforeUSBSync {
		fmt.Println("Deseja sincronizar o pendrive agora? [s/n]")
	} else {
		if err := a.Sync.SyncUSB(); err != nil {
			fmt.Println("Erro USB:", err)
		}
	}
}

func (a *App) handleUSBDisconnected() {
	fmt.Println("Pendrive removido de", a.Config.USBDriveLetter)
	if a.usbWatcher != nil {
		_ = a.usbWatcher.Close()
		a.usbWatcher = nil
	}
}

func (a *App) commandLoop() {
	go readInput(a.input)
	a.printCommands()
	for {
		fmt.Print("> ")
		select {
		case line := <-a.input:
			if !a.handleCommand(strings.TrimSpace(strings.ToLower(line))) {
				return
			}
		case connected := <-a.usbEvents:
			if connected {
				a.handleUSBConnected()
			} else {
				a.handleUSBDisconnected()
			}
		}
	}
}

func (a *App) handleCommand(command string) bool {
	switch command {
	case "s", "sim":
		if a.Config.IsPortable() {
			if err := a.Sync.SyncPortable(); err != nil {
				fmt.Println("Erro PORTABLE:", err)
			}
		} else {
			if err := a.Sync.SyncUSB(); err != nil {
				fmt.Println("Erro USB:", err)
			}
		}
	case "n", "nao", "não":
		fmt.Println("Sincronização ignorada.")
	case "sync":
		a.runImmediateSync()
	case "usb":
		if a.Config.IsPortable() {
			fmt.Println("No modo portable use sync.")
		} else if err := a.Sync.SyncUSB(); err != nil {
			fmt.Println("Erro USB:", err)
		}
	case "retry":
		if err := a.Sync.Retry(); err != nil {
			fmt.Println("Erro no retry:", err)
		}
	case "safe":
		if err := a.Sync.AddSafeDirectoryCurrent(); err != nil {
			fmt.Println("Erro safe.directory:", err)
		}
	case "status":
		a.printStatus()
	case "check":
		a.printGitCheck()
	case "unlock":
		a.Sync.UnlockAfterConflictResolved()
		fmt.Println("Bloqueio removido.")
	case "install-startup":
		if a.Config.IsPortable() {
			fmt.Println("Modo portable não pode instalar nada no PC.")
			break
		}
		exe, err := os.Executable()
		if err != nil {
			fmt.Println("Erro ao localizar Bella.exe:", err)
			break
		}
		if err := startup.InstallStartup(exe); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Inicialização automática instalada.")
		}
	case "uninstall-startup":
		if err := startup.UninstallStartup(); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Inicialização automática removida.")
		}
	case "config":
		a.printConfig()
	case "close":
		fmt.Println("Encerrando B.E.L.L.A...")
		return false
	case "":
		return true
	default:
		fmt.Println("Comando não reconhecido. Use: sync, usb, retry, safe, status, check, unlock, config, install-startup, uninstall-startup ou close")
	}
	return true
}

func (a *App) runImmediateSync() {
	if a.Config.IsPortable() {
		if err := a.Sync.SyncPortable(); err != nil {
			fmt.Println("Erro PORTABLE:", err)
		}
	} else {
		if err := a.Sync.SyncLocal(); err != nil {
			fmt.Println("Erro LOCAL:", err)
		}
	}
}

func (a *App) printHeader() {
	fmt.Println("B.E.L.L.A. v0.2.1 Clean Install")
	fmt.Println("Backup Engenhoso Ligeiramente Leve e Automático")
	fmt.Println("Modo:", a.Config.Mode)
	fmt.Println("Config:", a.ConfigPath)
	if a.Config.IsPortable() {
		fmt.Println("Repositório portable:", a.Config.RepositoryFolder)
	} else {
		fmt.Println("Pasta local:", a.Config.LocalFolder)
		fmt.Println("Pendrive:", a.Config.USBDriveLetter, a.Config.USBFolder)
	}
	fmt.Println("Git:", a.Config.GitPath)
	fmt.Println()
}

func (a *App) printCommands() {
	fmt.Println("Comandos: sync | usb | retry | safe | status | check | unlock | config | install-startup | uninstall-startup | close")
	fmt.Println("Use check para verificar alterações locais/remotas sem sincronizar.")
}

func (a *App) printStatus() {
	fmt.Println("Modo:", a.Config.Mode)
	fmt.Println("Branch:", a.Config.Branch)
	fmt.Println("Repo:", a.Config.RepoURL)
	fmt.Println("Git:", a.Config.GitPath)
	fmt.Println("Bloqueio por conflito:", a.Sync.IsBlockedByConflict())
	fmt.Println("Última falha:", a.Sync.LastFailed())
	if a.Config.IsPortable() {
		fmt.Println("Pasta portable:", a.Config.RepositoryFolder)
	} else {
		fmt.Println("Pasta local:", a.Config.LocalFolder)
		fmt.Println("Pendrive conectado:", usb.IsConnected(a.Config.USBDriveLetter))
		fmt.Println("Pasta USB:", a.Config.USBFolder)
	}
	fmt.Println("Último sync local:", a.State.Data.LastLocalSync)
	fmt.Println("Último sync USB:", a.State.Data.LastUSBSync)
	fmt.Println("Último sync portable:", a.State.Data.LastPortableSync)
	fmt.Println("Último erro:", a.State.Data.LastError)
}

func (a *App) printGitCheck() {
	if a.Config.IsPortable() {
		a.printRepoCheck("PORTABLE", a.Config.RepositoryFolder)
		return
	}
	a.printRepoCheck("LOCAL", a.Config.LocalFolder)
	if usb.IsConnected(a.Config.USBDriveLetter) {
		a.printRepoCheck("USB", a.Config.USBFolder)
	} else {
		fmt.Println("USB: não conectado")
	}
}

func (a *App) printRepoCheck(origin, folder string) {
	st, err := a.Sync.GetRepoStatus(folder)
	if err != nil {
		fmt.Println(origin+": erro ao verificar:", err)
		return
	}
	fmt.Println(origin + ":")
	fmt.Println("  Pasta:", folder)
	fmt.Println("  É repositório:", st.IsRepository)
	fmt.Println("  Alterações locais:", st.HasLocalChanges)
	fmt.Println("  Alterações remotas:", st.HasRemoteChanges)
	fmt.Println("  Commits para enviar:", st.HasCommitsToPush)
}

func (a *App) printConfig() {
	fmt.Println("Arquivo:", a.ConfigPath)
	fmt.Println("Modo:", a.Config.Mode)
	fmt.Println("Repo name:", a.Config.RepoName)
	fmt.Println("Repo URL:", a.Config.RepoURL)
	fmt.Println("Branch:", a.Config.Branch)
	fmt.Println("Delay:", a.Config.AutoCommitDelaySeconds, "segundos")
	fmt.Println("Limite:", a.Config.MaxFileSizeMB, "MB")
	fmt.Println("Git:", a.Config.GitPath)
	fmt.Println("Fallback Git:", a.Config.FallbackToSystemGit)
	fmt.Println("Sem salvar credenciais:", a.Config.NoSaveCredentials)
}

func (a *App) shutdown() {
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
	if a.localWatcher != nil {
		_ = a.localWatcher.Close()
	}
	if a.portableWatcher != nil {
		_ = a.portableWatcher.Close()
	}
	if a.usbWatcher != nil {
		_ = a.usbWatcher.Close()
	}
	if a.Log != nil {
		a.Log.Info("SYSTEM", "B.E.L.L.A. encerrada")
	}
}

func readInput(ch chan<- string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			continue
		}
		ch <- line
	}
}
func resolveAppPaths() (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	cfgWd := filepath.Join(wd, "config.json")
	if fileExists(cfgWd) {
		return wd, cfgWd, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Dir(exe)
	return dir, filepath.Join(dir, "config.json"), nil
}
func fileExists(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }
