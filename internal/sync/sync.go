package sync

import (
	"fmt"
	gosync "sync"
	"time"

	"bella/internal/config"
	bellagit "bella/internal/git"
	"bella/internal/ignore"
	"bella/internal/logger"
	"bella/internal/usb"
)

type SyncManager struct {
	Config    *config.Config
	GitClient *bellagit.Git
	Logger    *logger.Logger

	mutex             gosync.Mutex
	blockedByConflict bool
	isSyncing         bool
	pendingUSBSync    bool
}

func New(cfg *config.Config, gitClient *bellagit.Git, log *logger.Logger) *SyncManager {
	return &SyncManager{
		Config:            cfg,
		GitClient:         gitClient,
		Logger:            log,
		blockedByConflict: false,
		isSyncing:         false,
		pendingUSBSync:    false,
	}
}

func (s *SyncManager) SyncLocal() error {
	if err := s.beginSync("LOCAL"); err != nil {
		return err
	}

	defer s.endSync()

	s.Logger.Info("LOCAL", "Iniciando sincronização local")

	if err := s.checkLargeFilesAt("LOCAL", s.Config.LocalFolder); err != nil {
		return err
	}

	hasChanges, err := s.GitClient.HasChanges(s.Config.LocalFolder)
	if err != nil {
		s.Logger.Error("LOCAL", "Erro ao verificar alterações Git: "+err.Error())
		return err
	}

	if hasChanges {
		if err := s.GitClient.AddAll(s.Config.LocalFolder); err != nil {
			s.Logger.Error("LOCAL", "Erro ao executar git add: "+err.Error())
			return err
		}

		commitMessage := "BELLA sync local - " + time.Now().Format("2006-01-02 15:04:05")

		if err := s.GitClient.Commit(s.Config.LocalFolder, commitMessage); err != nil {
			s.Logger.Error("LOCAL", "Erro ao executar git commit: "+err.Error())
			return err
		}
	} else {
		s.Logger.Info("LOCAL", "Nenhuma alteração local encontrada")
		fmt.Println("Nenhuma alteração local para commitar.")
	}

	if err := s.GitClient.PullRebase(s.Config.LocalFolder, s.Config.Branch); err != nil {
		s.Logger.Error("LOCAL", "Erro ao executar git pull --rebase: "+err.Error())

		if bellagit.HasConflictMessage(err.Error()) {
			s.blockByConflict("LOCAL")
			return fmt.Errorf("conflito detectado durante pull --rebase. Resolva manualmente antes de continuar")
		}

		return err
	}

	if hasChanges {
		if err := s.GitClient.Push(s.Config.LocalFolder, s.Config.Branch); err != nil {
			s.Logger.Error("LOCAL", "Erro ao executar git push: "+err.Error())
			return err
		}
	}

	s.Logger.Success("LOCAL", "Sincronização local concluída com sucesso")
	fmt.Println("Sincronização local concluída com sucesso.")

	return nil
}

func (s *SyncManager) SyncUSB() error {
	if err := s.beginSync("USB"); err != nil {
		return err
	}

	defer s.endSync()

	s.Logger.Info("USB", "Iniciando sincronização do pendrive")

	if !usb.IsConnected(s.Config.USBDriveLetter) {
		message := "Pendrive não encontrado em " + s.Config.USBDriveLetter
		s.Logger.Error("USB", message)
		s.markPendingUSBSync(true)
		return fmt.Errorf(message)
	}

	if err := usb.EnsureUSBRepository(s.Config, s.GitClient, s.Logger); err != nil {
		s.Logger.Error("USB", "Erro ao preparar repositório no pendrive: "+err.Error())
		s.markPendingUSBSync(true)
		return err
	}

	if err := s.GitClient.AddSafeDirectory(s.Config.USBFolder); err != nil {
		s.Logger.Error("USB", "Erro ao registrar safe.directory: "+err.Error())
	}

	if err := s.checkLargeFilesAt("USB", s.Config.USBFolder); err != nil {
		return err
	}

	hasUSBChanges, err := s.GitClient.HasChanges(s.Config.USBFolder)
	if err != nil {
		if bellagit.IsSafeDirectoryError(err.Error()) {
			if safeErr := s.GitClient.AddSafeDirectory(s.Config.USBFolder); safeErr != nil {
				return fmt.Errorf("erro de safe.directory detectado e não foi possível corrigir: %w", safeErr)
			}

			hasUSBChanges, err = s.GitClient.HasChanges(s.Config.USBFolder)
		}

		if err != nil {
			s.Logger.Error("USB", "Erro ao verificar alterações Git no pendrive: "+err.Error())
			return err
		}
	}

	if hasUSBChanges {
		if err := s.GitClient.AddAll(s.Config.USBFolder); err != nil {
			s.Logger.Error("USB", "Erro ao executar git add no pendrive: "+err.Error())
			s.markPendingUSBSync(true)
			return err
		}

		commitMessage := "BELLA sync usb - " + time.Now().Format("2006-01-02 15:04:05")

		if err := s.GitClient.Commit(s.Config.USBFolder, commitMessage); err != nil {
			s.Logger.Error("USB", "Erro ao executar git commit no pendrive: "+err.Error())
			s.markPendingUSBSync(true)
			return err
		}
	} else {
		s.Logger.Info("USB", "Nenhuma alteração local encontrada no pendrive")
		fmt.Println("Nenhuma alteração no pendrive para commitar.")
	}

	if err := s.GitClient.PullRebase(s.Config.USBFolder, s.Config.Branch); err != nil {
		s.Logger.Error("USB", "Erro ao executar git pull --rebase no pendrive: "+err.Error())

		if bellagit.HasConflictMessage(err.Error()) {
			s.blockByConflict("USB")
			return fmt.Errorf("conflito detectado no pendrive durante pull --rebase. Resolva manualmente antes de continuar")
		}

		s.markPendingUSBSync(true)
		return err
	}

	if hasUSBChanges {
		if err := s.GitClient.Push(s.Config.USBFolder, s.Config.Branch); err != nil {
			s.Logger.Error("USB", "Erro ao executar git push no pendrive: "+err.Error())
			s.markPendingUSBSync(true)
			return err
		}
	}

	if err := s.updateLocalAfterUSB(); err != nil {
		return err
	}

	s.markPendingUSBSync(false)
	s.Logger.Success("USB", "Sincronização do pendrive concluída com sucesso")
	fmt.Println("Sincronização do pendrive concluída com sucesso.")

	return nil
}

func (s *SyncManager) updateLocalAfterUSB() error {
	hasLocalChanges, err := s.GitClient.HasChanges(s.Config.LocalFolder)
	if err != nil {
		s.Logger.Error("LOCAL", "Erro ao verificar pasta local após sincronização USB: "+err.Error())
		return err
	}

	if hasLocalChanges {
		message := "Pasta local possui alterações pendentes. Use o comando sync antes de atualizar após USB."
		s.Logger.Info("LOCAL", message)
		fmt.Println(message)
		return nil
	}

	if err := s.GitClient.PullRebase(s.Config.LocalFolder, s.Config.Branch); err != nil {
		s.Logger.Error("LOCAL", "Erro ao atualizar pasta local após USB: "+err.Error())

		if bellagit.HasConflictMessage(err.Error()) {
			s.blockByConflict("LOCAL")
			return fmt.Errorf("conflito detectado ao atualizar pasta local após USB")
		}

		return err
	}

	s.Logger.Success("LOCAL", "Pasta local atualizada após sincronização USB")
	return nil
}

func (s *SyncManager) beginSync(origin string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.blockedByConflict {
		message := "Sincronização bloqueada por conflito pendente. Resolva o conflito manualmente antes de tentar novamente."
		s.Logger.Blocked(origin, message)
		return fmt.Errorf(message)
	}

	if s.isSyncing {
		message := "Já existe uma sincronização em andamento."
		s.Logger.Info(origin, message)
		return fmt.Errorf(message)
	}

	s.isSyncing = true
	return nil
}

func (s *SyncManager) endSync() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.isSyncing = false
}

func (s *SyncManager) blockByConflict(origin string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.blockedByConflict = true
	s.Logger.Blocked(origin, "Conflito detectado. Novas sincronizações foram bloqueadas.")
}

func (s *SyncManager) IsBlockedByConflict() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.blockedByConflict
}

func (s *SyncManager) UnlockAfterConflictResolved() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.blockedByConflict = false
	s.Logger.Success("GIT", "Bloqueio por conflito removido manualmente")
}

func (s *SyncManager) IsSyncing() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.isSyncing
}

func (s *SyncManager) HasPendingUSBSync() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.pendingUSBSync
}

func (s *SyncManager) MarkPendingUSBSync(value bool) {
	s.markPendingUSBSync(value)
}

func (s *SyncManager) markPendingUSBSync(value bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pendingUSBSync = value
}

func (s *SyncManager) checkLargeFilesAt(origin string, folder string) error {
	largeFiles, err := ignore.FindLargeFiles(folder, s.Config.MaxFileSizeBytes())
	if err != nil {
		s.Logger.Error(origin, "Erro ao verificar arquivos grandes: "+err.Error())
		return err
	}

	if len(largeFiles) == 0 {
		return nil
	}

	for _, file := range largeFiles {
		message := fmt.Sprintf("Arquivo acima do limite ignorado: %s - %.2f MB", file.Path, file.SizeMB)
		s.Logger.Error(origin, message)
		fmt.Println(message)
	}

	return fmt.Errorf("existem arquivos acima do limite de %d MB", s.Config.MaxFileSizeMB)
}
