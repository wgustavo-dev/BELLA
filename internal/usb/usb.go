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
	drivePath := NormalizeDriveLetter(driveLetter)

	info, err := os.Stat(drivePath)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func NormalizeDriveLetter(driveLetter string) string {
	clean := strings.TrimSpace(driveLetter)

	if clean == "" {
		return clean
	}

	clean = strings.ReplaceAll(clean, "/", `\`)

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

	if bellagit.IsRepository(cfg.USBFolder) {
		log.Info("USB", "Pasta do pendrive já é um repositório Git")
		if err := gitClient.AddSafeDirectory(cfg.USBFolder); err != nil {
			log.Error("USB", "Não foi possível registrar safe.directory: "+err.Error())
		}
		return nil
	}

	parentDir := filepath.Dir(cfg.USBFolder)

	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return fmt.Errorf("erro ao criar pasta base do pendrive: %w", err)
	}

	if folderExists(cfg.USBFolder) {
		empty, err := isDirEmpty(cfg.USBFolder)
		if err != nil {
			return err
		}

		if empty {
			log.Info("USB", "Pasta USB vazia encontrada. Clonando repositório no pendrive.")

			if err := gitClient.Clone(cfg.RepoURL, cfg.Branch, cfg.USBFolder); err != nil {
				return err
			}

			_ = gitClient.AddSafeDirectory(cfg.USBFolder)
			log.Success("USB", "Repositório clonado no pendrive com sucesso")
			return nil
		}

		log.Info("USB", "Pasta USB não vazia e sem .git. Inicializando repositório local no pendrive.")

		if err := gitClient.InitRepository(cfg.USBFolder, cfg.RepoURL, cfg.Branch); err != nil {
			return err
		}

		_ = gitClient.AddSafeDirectory(cfg.USBFolder)
		log.Success("USB", "Repositório inicializado no pendrive com sucesso")
		return nil
	}

	log.Info("USB", "Pasta USB não existe. Clonando repositório no pendrive.")

	if err := gitClient.Clone(cfg.RepoURL, cfg.Branch, cfg.USBFolder); err != nil {
		return err
	}

	_ = gitClient.AddSafeDirectory(cfg.USBFolder)
	log.Success("USB", "Repositório clonado no pendrive com sucesso")
	return nil
}

func folderExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}

	return len(entries) == 0, nil
}
