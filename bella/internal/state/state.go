package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Data struct {
	Mode             string   `json:"mode"`
	LastLocalSync    string   `json:"lastLocalSync"`
	LastUSBSync      string   `json:"lastUsbSync"`
	LastPortableSync string   `json:"lastPortableSync"`
	LastError        string   `json:"lastError"`
	SafeDirectories  []string `json:"safeDirectories"`
}

type Store struct {
	path  string
	mutex sync.Mutex
	Data  Data
}

func New(appDir, mode string) (*Store, error) {
	dir := filepath.Join(appDir, ".bella")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, err
	}
	st := &Store{path: filepath.Join(dir, "state.json"), Data: Data{Mode: mode}}
	if data, err := os.ReadFile(st.path); err == nil {
		_ = json.Unmarshal(data, &st.Data)
	}
	st.Data.Mode = mode
	return st, st.save()
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) MarkSync(origin string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	now := time.Now().Format("2006-01-02 15:04:05")
	s.Data.LastError = ""
	switch origin {
	case "LOCAL":
		s.Data.LastLocalSync = now
	case "USB":
		s.Data.LastUSBSync = now
	case "PORTABLE":
		s.Data.LastPortableSync = now
	}
	_ = s.save()
}

func (s *Store) MarkError(msg string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Data.LastError = msg
	_ = s.save()
}

func (s *Store) AddSafeDirectory(path string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, item := range s.Data.SafeDirectories {
		if item == path {
			return
		}
	}
	s.Data.SafeDirectories = append(s.Data.SafeDirectories, path)
	_ = s.save()
}

func (s *Store) HasSafeDirectory(path string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, item := range s.Data.SafeDirectories {
		if item == path {
			return true
		}
	}
	return false
}
