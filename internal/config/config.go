package config

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

type Config struct {
	LocalFolder            string   `json:"localFolder"`
	RepoName               string   `json:"repoName"`
	RepoURL                string   `json:"repoUrl"`
	Branch                 string   `json:"branch"`
	AutoCommitDelaySeconds int      `json:"autoCommitDelaySeconds"`
	USBDriveLetter         string   `json:"usbDriveLetter"`
	USBFolder              string   `json:"usbFolder"`
	MaxFileSizeMB          int64    `json:"maxFileSizeMB"`
	GitPath                string   `json:"gitPath"`
	IgnorePatterns         []string `json:"ignorePatterns"`
	AutoStartWithWindows   bool     `json:"autoStartWithWindows"`
	AskBeforeUSBSync       bool     `json:"askBeforeUsbSync"`
}

func Load(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config

	if err := json.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	applyDefaults(&cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
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
	if cfg.LocalFolder == "" {
		return errors.New("localFolder não foi configurado")
	}

	if cfg.RepoName == "" {
		return errors.New("repoName não foi configurado")
	}

	if cfg.RepoURL == "" {
		return errors.New("repoUrl não foi configurado")
	}

	if cfg.Branch == "" {
		return errors.New("branch não foi configurado")
	}

	if cfg.USBDriveLetter == "" {
		return errors.New("usbDriveLetter não foi configurado")
	}

	if cfg.USBFolder == "" {
		return errors.New("usbFolder não foi configurado")
	}

	if cfg.MaxFileSizeMB <= 0 {
		return errors.New("maxFileSizeMB precisa ser maior que zero")
	}

	if cfg.GitPath == "" {
		return errors.New("gitPath não foi configurado")
	}

	return nil
}

func (cfg Config) AutoCommitDelay() time.Duration {
	return time.Duration(cfg.AutoCommitDelaySeconds) * time.Second
}

func (cfg Config) MaxFileSizeBytes() int64 {
	return cfg.MaxFileSizeMB * 1024 * 1024
}
