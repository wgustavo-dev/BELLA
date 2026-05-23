package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Mode                    string   `json:"mode"`
	LocalFolder             string   `json:"localFolder"`
	RepositoryFolder        string   `json:"repositoryFolder"`
	RepoName                string   `json:"repoName"`
	RepoURL                 string   `json:"repoUrl"`
	Branch                  string   `json:"branch"`
	AutoCommitDelaySeconds  int      `json:"autoCommitDelaySeconds"`
	USBDriveLetter          string   `json:"usbDriveLetter"`
	USBFolder               string   `json:"usbFolder"`
	MaxFileSizeMB           int64    `json:"maxFileSizeMB"`
	GitPath                 string   `json:"gitPath"`
	FallbackToSystemGit     bool     `json:"fallbackToSystemGit"`
	AskBeforeUSBSync        bool     `json:"askBeforeUsbSync"`
	AskBeforePortableSync   bool     `json:"askBeforePortableSync"`
	SyncOnStart             bool     `json:"syncOnStart"`
	AutoStartWithWindows    bool     `json:"autoStartWithWindows"`
	NoSaveCredentials       bool     `json:"noCredentialSave"`
	SafeDirectoryPromptOnce bool     `json:"safeDirectoryPromptOnce"`
	GitUserName             string   `json:"gitUserName"`
	GitUserEmail            string   `json:"gitUserEmail"`
	IgnorePatterns          []string `json:"ignorePatterns"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	return &cfg, validate(cfg)
}

func LoadOrCreateFirstRun(path string) (*Config, bool, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, true, nil
	}
	cfg, err := Load(path)
	return cfg, false, err
}

func applyDefaults(cfg *Config) {
	if cfg.Mode == "" {
		cfg.Mode = "installed"
	}
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.AutoCommitDelaySeconds <= 0 {
		cfg.AutoCommitDelaySeconds = 30
	}
	if cfg.MaxFileSizeMB <= 0 {
		cfg.MaxFileSizeMB = 100
	}
	if cfg.GitPath == "" {
		cfg.GitPath = "git"
	}
	if cfg.IgnorePatterns == nil {
		cfg.IgnorePatterns = []string{}
	}
}

func validate(cfg Config) error {
	if cfg.Mode != "installed" && cfg.Mode != "portable" {
		return errors.New("mode deve ser installed ou portable")
	}
	if cfg.RepoName == "" {
		return errors.New("repoName não foi configurado")
	}
	if cfg.RepoURL == "" {
		return errors.New("repoUrl não foi configurado")
	}
	if cfg.Branch == "" {
		return errors.New("branch não foi configurada")
	}
	if cfg.IsInstalled() {
		if cfg.LocalFolder == "" {
			return errors.New("localFolder não foi configurado")
		}
		if cfg.USBDriveLetter == "" {
			return errors.New("usbDriveLetter não foi configurado")
		}
		if cfg.USBFolder == "" {
			return errors.New("usbFolder não foi configurado")
		}
	}
	if cfg.IsPortable() && cfg.RepositoryFolder == "" {
		return errors.New("repositoryFolder não foi configurado")
	}
	return nil
}

// ResolvePaths transforma caminhos relativos em caminhos absolutos com base na pasta do Bella.exe.
// Isso é o que permite o modo portable funcionar em D:, E:, F: etc. sem editar o config.json.
func (cfg *Config) ResolvePaths(appDir string) error {
	var err error
	if cfg.LocalFolder != "" {
		cfg.LocalFolder, err = resolvePath(appDir, cfg.LocalFolder)
		if err != nil {
			return err
		}
	}
	if cfg.RepositoryFolder != "" {
		cfg.RepositoryFolder, err = resolvePath(appDir, cfg.RepositoryFolder)
		if err != nil {
			return err
		}
	}
	if cfg.USBFolder != "" {
		cfg.USBFolder, err = resolvePath(appDir, cfg.USBFolder)
		if err != nil {
			return err
		}
	}
	if cfg.GitPath != "" && cfg.GitPath != "git" {
		cfg.GitPath, err = resolvePath(appDir, cfg.GitPath)
		if err != nil {
			return err
		}
	}
	if cfg.IsPortable() {
		cfg.LocalFolder = cfg.RepositoryFolder
	}
	return nil
}

func resolvePath(base, path string) (string, error) {
	if filepath.IsAbs(path) || path == "git" {
		return path, nil
	}
	return filepath.Abs(filepath.Join(base, path))
}

func (cfg Config) IsInstalled() bool { return cfg.Mode == "installed" }
func (cfg Config) IsPortable() bool  { return cfg.Mode == "portable" }
func (cfg Config) AutoCommitDelay() time.Duration {
	return time.Duration(cfg.AutoCommitDelaySeconds) * time.Second
}
func (cfg Config) MaxFileSizeBytes() int64 { return cfg.MaxFileSizeMB * 1024 * 1024 }
