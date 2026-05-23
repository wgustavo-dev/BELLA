package usb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bella/internal/config"
	bellagit "bella/internal/git"
	"bella/internal/logger"
)

func IsConnected(driveLetter string) bool {
	p := NormalizeDriveLetter(driveLetter)
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func NormalizeDriveLetter(driveLetter string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(driveLetter, "/", `\`))
	if clean == "" {
		return clean
	}
	if strings.HasSuffix(clean, `\`) {
		return clean
	}
	if strings.HasSuffix(clean, ":") {
		return clean + `\`
	}
	return clean
}

func EnsureUSBRepository(cfg *config.Config, gitClient *bellagit.Git, log *logger.Logger) error {
	if !IsConnected(cfg.USBDriveLetter) {
		return fmt.Errorf("pendrive não encontrado em %s", cfg.USBDriveLetter)
	}
	return EnsureRepository(cfg.USBFolder, cfg.RepoURL, cfg.Branch, gitClient, log, "USB")
}

// EnsureRepository prepara uma pasta para ser repositório Git.
// Se a pasta não existir ou estiver vazia, clona. Se já tiver arquivos, inicializa sem apagar nada.
func EnsureRepository(folder, repoURL, branch string, gitClient *bellagit.Git, log *logger.Logger, origin string) error {
	if bellagit.IsRepository(folder) {
		_ = gitClient.SetUpstream(folder, branch)
		return nil
	}
	if exists(folder) {
		empty, err := isEmpty(folder)
		if err != nil {
			return err
		}
		if empty {
			log.Info(origin, "Pasta vazia. Clonando repositório.")
			return gitClient.Clone(repoURL, branch, folder)
		}
		log.Warn(origin, "Pasta com arquivos e sem .git. Inicializando repositório sem apagar arquivos.")
		return gitClient.InitRepository(folder, repoURL, branch)
	}
	if err := os.MkdirAll(filepath.Dir(folder), os.ModePerm); err != nil {
		return err
	}
	log.Info(origin, "Pasta não existe. Clonando repositório.")
	return gitClient.Clone(repoURL, branch, folder)
}

func exists(path string) bool { info, err := os.Stat(path); return err == nil && info.IsDir() }
func isEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
