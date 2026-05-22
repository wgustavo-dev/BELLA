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
	bellasync "bella/internal/sync"
	"bella/internal/usb"
	bellawatcher "bella/internal/watcher"
)

type appEvent struct {
	Kind    string
	Message string
}

const (
	eventUSBConnected    = "usb_connected"
	eventUSBDisconnected = "usb_disconnected"
	eventUSBError        = "usb_error"
)

func main() {
	fmt.Println("B.E.L.L.A. - Backup Engenhoso Ligeiramente Leve e Automático")
	fmt.Println("Iniciando aplicação...")

	appDir, configPath, err := resolveAppPaths()
	if err != nil {
		fmt.Println("Erro ao localizar diretório da aplicação:", err)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Println("Erro ao carregar config.json:", err)
		fmt.Println("Verifique se o arquivo config.json existe na pasta do projeto ou ao lado do Bella.exe.")
		return
	}

	log, err := logger.New(appDir)
	if err != nil {
		fmt.Println("Erro ao iniciar logger:", err)
		return
	}

	log.Info("SYSTEM", "B.E.L.L.A. iniciada")
	log.Info("CONFIG", "Configuração carregada com sucesso")

	gitClient := bellagit.New(cfg.GitPath)

	if !gitClient.IsGitInstalled() {
		fmt.Println("Git não encontrado.")
		fmt.Println("Verifique se o Git está instalado ou ajuste o campo gitPath no config.json.")
		log.Error("GIT", "Git não encontrado no caminho configurado: "+cfg.GitPath)
		return
	}

	log.Success("GIT", "Git encontrado com sucesso")

	if err := prepareLocal(cfg, gitClient, log); err != nil {
		fmt.Println("Erro ao preparar pasta local:", err)
		return
	}

	printStartupInfo(appDir, configPath, cfg)

	syncManager := bellasync.New(cfg, gitClient, log)

	localWatcher := bellawatcher.New("LOCAL", cfg.LocalFolder, cfg.AutoCommitDelay(), log, func() error {
		return syncManager.SyncLocal()
	})

	if err := localWatcher.Start(); err != nil {
		fmt.Println("Erro ao iniciar monitoramento da pasta local:", err)
		log.Error("WATCHER", "Erro ao iniciar monitoramento local: "+err.Error())
		return
	}

	defer localWatcher.Close()

	events := make(chan appEvent, 20)
	stopUSBMonitor := make(chan struct{})

	go startUSBMonitor(cfg, gitClient, log, syncManager, events, stopUSBMonitor)

	defer close(stopUSBMonitor)

	fmt.Println()
	fmt.Println("Monitoramento automático iniciado.")
	fmt.Println("A B.E.L.L.A. está observando:", cfg.LocalFolder)
	fmt.Println("Tempo de espera antes da sincronização:", cfg.AutoCommitDelaySeconds, "segundos")

	if usb.IsConnected(cfg.USBDriveLetter) {
		fmt.Println("Pendrive detectado em:", cfg.USBDriveLetter)
		fmt.Println("Digite 'usb' para sincronizar agora.")
	} else {
		fmt.Println("Pendrive não detectado em:", cfg.USBDriveLetter)
	}

	printCommands()

	waitForCommands(log, syncManager, cfg, events)
}

func prepareLocal(cfg *config.Config, gitClient *bellagit.Git, log *logger.Logger) error {
	if err := os.MkdirAll(cfg.LocalFolder, os.ModePerm); err != nil {
		log.Error("LOCAL", "Erro ao criar pasta local: "+err.Error())
		return err
	}

	if err := ignore.EnsureGitignore(cfg.LocalFolder, cfg.IgnorePatterns); err != nil {
		log.Error("IGNORE", "Erro ao preparar .gitignore: "+err.Error())
		return err
	}

	gitignorePath := filepath.Join(cfg.LocalFolder, ".gitignore")
	fmt.Println(".gitignore criado/verificado em:", gitignorePath)
	log.Success("IGNORE", ".gitignore preparado com sucesso")

	largeFiles, err := ignore.FindLargeFiles(cfg.LocalFolder, cfg.MaxFileSizeBytes())
	if err != nil {
		log.Error("IGNORE", "Erro ao verificar arquivos grandes: "+err.Error())
		return err
	}

	if bellagit.IsRepository(cfg.LocalFolder) {
		fmt.Println("Status Git: pasta local já é um repositório.")
		log.Info("GIT", "Pasta local já é um repositório Git")
	} else {
		fmt.Println("Status Git: pasta local ainda não é um repositório.")
		fmt.Println("Preparando repositório local...")

		log.Info("GIT", "Pasta local ainda não é um repositório Git")

		if err := gitClient.InitRepository(cfg.LocalFolder, cfg.RepoURL, cfg.Branch); err != nil {
			log.Error("GIT", "Erro ao preparar repositório local: "+err.Error())
			return err
		}

		fmt.Println("Repositório local preparado com sucesso.")
		log.Success("GIT", "Repositório local preparado com sucesso")
	}

	if len(largeFiles) > 0 {
		fmt.Println()
		fmt.Println("Arquivos grandes detectados:")

		for _, file := range largeFiles {
			message := fmt.Sprintf("%s - %.2f MB", file.Path, file.SizeMB)
			fmt.Println("-", message)
			log.Error("IGNORE", "Arquivo acima do limite: "+message)
		}

		fmt.Println("Esses arquivos não devem ser versionados automaticamente.")
	} else {
		fmt.Println("Arquivos grandes: nenhum encontrado.")
		log.Info("IGNORE", "Nenhum arquivo acima do limite encontrado")
	}

	return nil
}

func printStartupInfo(appDir string, configPath string, cfg *config.Config) {
	fmt.Println("Configuração carregada de:", configPath)
	fmt.Println("Pasta interna da B.E.L.L.A.:", appDir)
	fmt.Println("Pasta local monitorada:", cfg.LocalFolder)
	fmt.Println("Repositório:", cfg.RepoName)
	fmt.Println("Branch:", cfg.Branch)
	fmt.Println("Pendrive configurado:", cfg.USBDriveLetter)
	fmt.Println("Pasta no pendrive:", cfg.USBFolder)
	fmt.Println("Git:", cfg.GitPath)
	fmt.Println("Limite de arquivo:", cfg.MaxFileSizeMB, "MB")
	fmt.Println("Perguntar antes de sincronizar USB:", cfg.AskBeforeUSBSync)
	fmt.Println("Inicialização com Windows configurada:", startup.IsStartupInstalled())
}

func printCommands() {
	fmt.Println()
	fmt.Println("Comandos disponíveis:")
	fmt.Println("- sync              -> sincronizar pasta local com GitHub manualmente")
	fmt.Println("- usb               -> sincronizar pendrive com GitHub")
	fmt.Println("- status            -> mostrar status interno da B.E.L.L.A.")
	fmt.Println("- unlock            -> liberar bloqueio de conflito após correção manual")
	fmt.Println("- install-startup   -> iniciar B.E.L.L.A. junto com o Windows")
	fmt.Println("- uninstall-startup -> remover B.E.L.L.A. da inicialização do Windows")
	fmt.Println("- close             -> encerrar a B.E.L.L.A.")
}

func startUSBMonitor(cfg *config.Config, gitClient *bellagit.Git, log *logger.Logger, syncManager *bellasync.SyncManager, events chan<- appEvent, stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var usbWatcher *bellawatcher.FolderWatcher
	lastConnected := false

	handleConnected := func() {
		if !usb.IsConnected(cfg.USBDriveLetter) {
			return
		}

		lastConnected = true

		if err := usb.EnsureUSBRepository(cfg, gitClient, log); err != nil {
			message := "Erro ao preparar pendrive: " + err.Error()
			log.Error("USB", message)
			events <- appEvent{Kind: eventUSBError, Message: message}
			return
		}

		if usbWatcher == nil {
			usbWatcher = bellawatcher.New("USB", cfg.USBFolder, cfg.AutoCommitDelay(), log, func() error {
				return syncManager.SyncUSB()
			})

			if err := usbWatcher.Start(); err != nil {
				message := "Erro ao iniciar watcher do pendrive: " + err.Error()
				log.Error("USB", message)
				events <- appEvent{Kind: eventUSBError, Message: message}
				usbWatcher = nil
				return
			}
		}

		log.Info("USB", "Pendrive detectado em "+cfg.USBDriveLetter)
		events <- appEvent{
			Kind:    eventUSBConnected,
			Message: "Pendrive detectado em " + cfg.USBDriveLetter,
		}

		if !cfg.AskBeforeUSBSync || syncManager.HasPendingUSBSync() {
			go func() {
				if err := syncManager.SyncUSB(); err != nil {
					log.Error("USB", "Erro na sincronização automática ao conectar: "+err.Error())
				}
			}()
		}
	}

	handleDisconnected := func() {
		lastConnected = false

		if usbWatcher != nil {
			_ = usbWatcher.Close()
			usbWatcher = nil
		}

		syncManager.MarkPendingUSBSync(true)

		log.Error("USB", "Pendrive removido de "+cfg.USBDriveLetter)
		events <- appEvent{
			Kind:    eventUSBDisconnected,
			Message: "Pendrive removido de " + cfg.USBDriveLetter,
		}
	}

	if usb.IsConnected(cfg.USBDriveLetter) {
		handleConnected()
	}

	for {
		select {
		case <-stop:
			if usbWatcher != nil {
				_ = usbWatcher.Close()
			}
			return

		case <-ticker.C:
			currentConnected := usb.IsConnected(cfg.USBDriveLetter)

			if currentConnected && !lastConnected {
				handleConnected()
				continue
			}

			if !currentConnected && lastConnected {
				handleDisconnected()
				continue
			}
		}
	}
}

func resolveAppPaths() (string, string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	configInWorkingDir := filepath.Join(workingDir, "config.json")

	if fileExists(configInWorkingDir) {
		return workingDir, configInWorkingDir, nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", "", err
	}

	execDir := filepath.Dir(execPath)
	configInExecDir := filepath.Join(execDir, "config.json")

	if fileExists(configInExecDir) {
		return execDir, configInExecDir, nil
	}

	return workingDir, configInWorkingDir, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)

	if err != nil {
		return false
	}

	return !info.IsDir()
}

func readCommands(input chan<- string) {
	reader := bufio.NewReader(os.Stdin)

	for {
		inputText, err := reader.ReadString('\n')
		if err != nil {
			continue
		}

		input <- strings.TrimSpace(strings.ToLower(inputText))
	}
}

func waitForCommands(log *logger.Logger, syncManager *bellasync.SyncManager, cfg *config.Config, events <-chan appEvent) {
	input := make(chan string)
	go readCommands(input)

	pendingUSBQuestion := false

	for {
		fmt.Print("> ")

		select {
		case event := <-events:
			fmt.Println()

			switch event.Kind {
			case eventUSBConnected:
				fmt.Println(event.Message)

				if cfg.AskBeforeUSBSync && !syncManager.HasPendingUSBSync() {
					fmt.Println("Deseja sincronizar agora? [s/n]")
					pendingUSBQuestion = true
				} else if cfg.AskBeforeUSBSync && syncManager.HasPendingUSBSync() {
					fmt.Println("Existe sincronização USB pendente. Deseja tentar agora? [s/n]")
					pendingUSBQuestion = true
				}

			case eventUSBDisconnected:
				fmt.Println(event.Message)
				fmt.Println("A B.E.L.L.A. tentará novamente quando o pendrive for reconectado.")

			case eventUSBError:
				fmt.Println(event.Message)
			}

		case command := <-input:
			if pendingUSBQuestion {
				switch command {
				case "s", "sim", "y", "yes":
					pendingUSBQuestion = false
					fmt.Println("Sincronizando pendrive com GitHub...")

					if err := syncManager.SyncUSB(); err != nil {
						fmt.Println("Erro na sincronização USB:", err)
						log.Error("USB", "Erro na sincronização USB por confirmação: "+err.Error())
					}

					continue

				case "n", "nao", "não", "no":
					pendingUSBQuestion = false
					fmt.Println("Sincronização USB ignorada por enquanto.")
					log.Info("USB", "Usuário recusou sincronização USB no momento")
					continue
				}
			}

			switch command {
			case "sync":
				fmt.Println("Sincronizando pasta local com GitHub...")

				if err := syncManager.SyncLocal(); err != nil {
					fmt.Println("Erro na sincronização:", err)
					log.Error("LOCAL", "Erro na sincronização manual: "+err.Error())
				}

			case "usb":
				if !usb.IsConnected(cfg.USBDriveLetter) {
					fmt.Println("Pendrive não encontrado em:", cfg.USBDriveLetter)
					log.Error("USB", "Comando usb chamado, mas pendrive não encontrado")
					continue
				}

				fmt.Println("Sincronizando pendrive com GitHub...")

				if err := syncManager.SyncUSB(); err != nil {
					fmt.Println("Erro na sincronização USB:", err)
					log.Error("USB", "Erro na sincronização manual USB: "+err.Error())
				}

			case "status":
				printStatus(syncManager, cfg)

			case "unlock":
				syncManager.UnlockAfterConflictResolved()
				fmt.Println("Bloqueio removido. Use 'sync' ou 'usb' para testar a sincronização novamente.")

			case "install-startup":
				exePath, err := startup.CurrentExecutablePath()
				if err != nil {
					fmt.Println("Erro ao localizar Bella.exe:", err)
					continue
				}

				if err := startup.InstallStartup(exePath); err != nil {
					fmt.Println("Erro ao instalar inicialização automática:", err)
					log.Error("STARTUP", "Erro ao instalar inicialização automática: "+err.Error())
					continue
				}

				fmt.Println("B.E.L.L.A. configurada para iniciar com o Windows.")
				log.Success("STARTUP", "Inicialização automática instalada")

			case "uninstall-startup":
				if err := startup.UninstallStartup(); err != nil {
					fmt.Println("Erro ao remover inicialização automática:", err)
					log.Error("STARTUP", "Erro ao remover inicialização automática: "+err.Error())
					continue
				}

				fmt.Println("B.E.L.L.A. removida da inicialização do Windows.")
				log.Success("STARTUP", "Inicialização automática removida")

			case "close":
				fmt.Println("Encerrando B.E.L.L.A...")
				log.Info("SYSTEM", "B.E.L.L.A. encerrada pelo usuário")
				return

			case "":
				continue

			default:
				fmt.Println("Comando não reconhecido. Use: sync, usb, status, unlock, install-startup, uninstall-startup ou close")
			}
		}
	}
}

func printStatus(syncManager *bellasync.SyncManager, cfg *config.Config) {
	if syncManager.IsBlockedByConflict() {
		fmt.Println("Status: bloqueada por conflito Git.")
	} else if syncManager.IsSyncing() {
		fmt.Println("Status: sincronização em andamento.")
	} else {
		fmt.Println("Status: pronta para sincronizar.")
	}

	if usb.IsConnected(cfg.USBDriveLetter) {
		fmt.Println("Pendrive: conectado em", cfg.USBDriveLetter)
	} else {
		fmt.Println("Pendrive: não conectado em", cfg.USBDriveLetter)
	}

	if syncManager.HasPendingUSBSync() {
		fmt.Println("USB pendente: sim")
	} else {
		fmt.Println("USB pendente: não")
	}

	fmt.Println("Pasta local:", cfg.LocalFolder)
	fmt.Println("Pasta USB:", cfg.USBFolder)
	fmt.Println("Branch:", cfg.Branch)
	fmt.Println("Repositório:", cfg.RepoURL)
	fmt.Println("Inicialização com Windows:", startup.IsStartupInstalled())
}
