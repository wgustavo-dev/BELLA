package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	filePath string
	mutex    sync.Mutex
}

func New(appDir string) (*Logger, error) {
	logsDir := filepath.Join(appDir, ".bella", "logs")
	if err := os.MkdirAll(logsDir, os.ModePerm); err != nil {
		return nil, err
	}
	filePath := filepath.Join(logsDir, "sync.log")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	file.Close()
	return &Logger{filePath: filePath}, nil
}

// Write salva uma linha padronizada no log da B.E.L.L.A.
// O mutex evita que duas rotinas escrevam no arquivo ao mesmo tempo.
func (l *Logger) Write(level, origin, message string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	line := fmt.Sprintf("[%s] [%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, origin, message)
	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Erro ao escrever log:", err)
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line)
}

func (l *Logger) Info(origin, message string)    { l.Write("INFO", origin, message) }
func (l *Logger) Success(origin, message string) { l.Write("SUCCESS", origin, message) }
func (l *Logger) Warn(origin, message string)    { l.Write("WARN", origin, message) }
func (l *Logger) Error(origin, message string)   { l.Write("ERROR", origin, message) }
func (l *Logger) Blocked(origin, message string) { l.Write("BLOCKED", origin, message) }
