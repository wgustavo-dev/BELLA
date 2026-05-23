package sync

import (
	"fmt"
	gosync "sync"
	"time"

	"bella/internal/config"
	bellagit "bella/internal/git"
	"bella/internal/ignore"
	"bella/internal/logger"
	"bella/internal/state"
	"bella/internal/usb"
)

type SyncManager struct {
	Config     *config.Config
	GitClient  *bellagit.Git
	Logger     *logger.Logger
	State      *state.Store
	mutex      gosync.Mutex
	blocked    bool
	syncing    bool
	lastFailed string
}

func New(cfg *config.Config, gitClient *bellagit.Git, log *logger.Logger, st *state.Store) *SyncManager {
	return &SyncManager{Config: cfg, GitClient: gitClient, Logger: log, State: st}
}
func (s *SyncManager) SyncLocal() error {
	return s.syncRepo("LOCAL", s.Config.LocalFolder, "BELLA sync local - ")
}
func (s *SyncManager) SyncPortable() error {
	return s.syncRepo("PORTABLE", s.Config.RepositoryFolder, "BELLA sync portable - ")
}

func (s *SyncManager) SyncUSB() error {
	if err := s.begin("USB"); err != nil {
		return err
	}
	defer s.end()
	if !usb.IsConnected(s.Config.USBDriveLetter) {
		return s.fail("USB", "pendrive não encontrado em "+s.Config.USBDriveLetter)
	}
	if err := usb.EnsureUSBRepository(s.Config, s.GitClient, s.Logger); err != nil {
		return s.fail("USB", err.Error())
	}
	if err := s.syncRepoUnlocked("USB", s.Config.USBFolder, "BELLA sync usb - "); err != nil {
		return err
	}
	return s.updateLocalAfterUSB()
}

func (s *SyncManager) syncRepo(origin, folder, prefix string) error {
	if err := s.begin(origin); err != nil {
		return err
	}
	defer s.end()
	return s.syncRepoUnlocked(origin, folder, prefix)
}

// syncRepoUnlocked é o coração da sincronização.
// Ele verifica alterações locais, alterações remotas e commits pendentes antes de decidir o que fazer.
func (s *SyncManager) syncRepoUnlocked(origin, folder, prefix string) error {
	s.Logger.Info(origin, "Verificando alterações em: "+folder)
	if err := s.GitClient.EnsureUserIdentity(folder, s.Config.GitUserName, s.Config.GitUserEmail); err != nil {
		return s.fail(origin, err.Error())
	}
	ignoredLargeFiles, err := s.handleLargeFiles(origin, folder)
	if err != nil {
		s.lastFailed = origin
		s.State.MarkError(err.Error())
		return err
	}

	st, err := s.GitClient.GetRepoStatus(folder, s.Config.Branch)
	if err != nil {
		if bellagit.HasDubiousOwnership(err.Error()) {
			return s.safeDirectoryAndRetry(origin, folder)
		}
		return s.fail(origin, err.Error())
	}
	if !st.IsRepository {
		return s.fail(origin, "pasta não é repositório Git: "+folder)
	}

	if st.HasLocalChanges {
		fmt.Println(origin + ": alterações locais encontradas. Criando commit...")
		if err := s.GitClient.AddAll(folder); err != nil {
			return s.fail(origin, err.Error())
		}
		if err := s.GitClient.Commit(folder, prefix+time.Now().Format("2006-01-02 15:04:05")); err != nil {
			return s.fail(origin, err.Error())
		}
	} else {
		fmt.Println(origin + ": nenhuma alteração local.")
	}

	// Depois do commit, checamos de novo o remoto e commits para push.
	if err := s.GitClient.Fetch(folder, s.Config.Branch); err != nil {
		return s.fail(origin, err.Error())
	}
	remote, _ := s.GitClient.HasRemoteChanges(folder, s.Config.Branch)
	if remote {
		fmt.Println(origin + ": GitHub tem alterações novas. Aplicando rebase...")
		if err := s.GitClient.PullRebase(folder, s.Config.Branch); err != nil {
			if bellagit.HasConflictMessage(err.Error()) {
				s.block(origin)
				return fmt.Errorf("conflito detectado em %s. Resolva manualmente e use unlock", origin)
			}
			return s.fail(origin, err.Error())
		}
	}
	push, _ := s.GitClient.HasCommitsToPush(folder, s.Config.Branch)
	if push {
		fmt.Println(origin + ": enviando commits para o GitHub...")
		if err := s.GitClient.Push(folder, s.Config.Branch); err != nil {
			return s.fail(origin, err.Error())
		}
	}
	if !st.HasLocalChanges && !remote && !push && len(ignoredLargeFiles) == 0 {
		fmt.Println(origin + ": já estava sincronizado.")
	}

	s.Logger.Success(origin, "Sincronização concluída")
	s.State.MarkSync(origin)
	s.lastFailed = ""
	fmt.Println("Sincronização", origin, "concluída com sucesso.")
	s.printLargeFileReport(origin, ignoredLargeFiles)
	return nil
}

// handleLargeFiles não bloqueia mais a sincronização.
// Ele adiciona os arquivos grandes ao .gitignore e remove do controle Git se já estiverem rastreados.
func (s *SyncManager) handleLargeFiles(origin, folder string) ([]ignore.LargeFile, error) {
	files, err := ignore.FindLargeFiles(folder, s.Config.MaxFileSizeBytes())
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	var patterns []string
	fmt.Println("\nArquivos grandes detectados e ignorados automaticamente:")
	for _, file := range files {
		patterns = append(patterns, file.Path)
		msg := fmt.Sprintf("%s - %.2f MB", file.Path, file.SizeMB)
		fmt.Println("-", msg)
		s.Logger.Warn(origin, "Arquivo grande detectado: "+msg)
		if s.GitClient.IsFileTracked(folder, file.Path) {
			if err := s.GitClient.RemoveCached(folder, file.Path); err != nil {
				return files, err
			}
			s.Logger.Info(origin, "Arquivo grande removido do controle Git: "+file.Path)
		}
	}
	if err := ignore.AddPatternsToGitignore(folder, patterns); err != nil {
		return files, err
	}
	if err := s.GitClient.AddFile(folder, ".gitignore"); err != nil {
		return files, err
	}
	fmt.Println("Esses arquivos foram adicionados ao .gitignore. A sincronização continuará sem eles.\n")
	return files, nil
}

func (s *SyncManager) printLargeFileReport(origin string, files []ignore.LargeFile) {
	if len(files) == 0 {
		return
	}
	fmt.Println("\nRelatório da B.E.L.L.A.: arquivos grandes ignorados nesta sincronização:")
	for _, f := range files {
		fmt.Printf("- %s - %.2f MB\n", f.Path, f.SizeMB)
		s.Logger.Info(origin, fmt.Sprintf("Relatório: arquivo grande ignorado: %s - %.2f MB", f.Path, f.SizeMB))
	}
	fmt.Println("Eles continuam no dispositivo, mas não serão enviados ao GitHub.\n")
}

func (s *SyncManager) GetRepoStatus(folder string) (*bellagit.RepoStatus, error) {
	return s.GitClient.GetRepoStatus(folder, s.Config.Branch)
}
func (s *SyncManager) safeDirectoryAndRetry(origin, folder string) error {
	fmt.Println("safe.directory necessário para:", folder)
	s.lastFailed = origin
	s.State.MarkError("safe.directory necessário: " + folder)
	return fmt.Errorf("safe.directory necessário. Use o comando safe e depois retry")
}
func (s *SyncManager) AddSafeDirectoryCurrent() error {
	folder := s.Config.LocalFolder
	if s.Config.IsPortable() {
		folder = s.Config.RepositoryFolder
	}
	if s.lastFailed == "USB" {
		folder = s.Config.USBFolder
	}
	if err := s.GitClient.AddSafeDirectory(folder); err != nil {
		return err
	}
	s.State.AddSafeDirectory(folder)
	fmt.Println("safe.directory configurado para:", folder)
	return nil
}
func (s *SyncManager) Retry() error {
	switch s.lastFailed {
	case "LOCAL":
		return s.SyncLocal()
	case "USB":
		return s.SyncUSB()
	case "PORTABLE":
		return s.SyncPortable()
	default:
		return fmt.Errorf("não existe sincronização pendente")
	}
}

func (s *SyncManager) updateLocalAfterUSB() error {
	has, err := s.GitClient.HasChanges(s.Config.LocalFolder)
	if err != nil {
		return s.fail("LOCAL", err.Error())
	}
	if has {
		fmt.Println("Pasta local possui alterações pendentes. Use sync antes de atualizar após USB.")
		return nil
	}
	if err := s.GitClient.PullRebase(s.Config.LocalFolder, s.Config.Branch); err != nil {
		if bellagit.HasConflictMessage(err.Error()) {
			s.block("LOCAL")
		}
		return s.fail("LOCAL", err.Error())
	}
	s.State.MarkSync("LOCAL")
	return nil
}
func (s *SyncManager) begin(origin string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.blocked {
		return fmt.Errorf("sincronização bloqueada por conflito. Resolva e use unlock")
	}
	if s.syncing {
		return fmt.Errorf("já existe uma sincronização em andamento")
	}
	s.syncing = true
	return nil
}
func (s *SyncManager) end() { s.mutex.Lock(); defer s.mutex.Unlock(); s.syncing = false }
func (s *SyncManager) block(origin string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.blocked = true
	s.lastFailed = origin
	s.Logger.Blocked(origin, "Conflito detectado")
}
func (s *SyncManager) IsBlockedByConflict() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.blocked
}
func (s *SyncManager) UnlockAfterConflictResolved() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.blocked = false
	s.Logger.Success("GIT", "Bloqueio removido manualmente")
}
func (s *SyncManager) LastFailed() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.lastFailed
}
func (s *SyncManager) fail(origin, msg string) error {
	s.Logger.Error(origin, msg)
	s.State.MarkError(msg)
	s.lastFailed = origin
	return fmt.Errorf("%s", msg)
}
