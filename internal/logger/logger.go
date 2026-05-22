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

func New(baseDir string) (*Logger, error) {
	logsDir := filepath.Join(baseDir, ".bella", "logs")

	if err := os.MkdirAll(logsDir, os.ModePerm); err != nil {
		return nil, err
	}

	lockPath := filepath.Join(baseDir, ".bella", "bella.lock")
	if _, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644); err != nil {
		return nil, err
	}

	filePath := filepath.Join(logsDir, "sync.log")

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	file.Close()

	return &Logger{
		filePath: filePath,
	}, nil
}

func (l *Logger) Write(level string, origin string, message string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] [%s] %s\n", now, level, origin, message)

	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Erro ao escrever log:", err)
		return
	}

	defer file.Close()

	if _, err := file.WriteString(line); err != nil {
		fmt.Println("Erro ao salvar log:", err)
	}
}

func (l *Logger) Info(origin string, message string) {
	l.Write("INFO", origin, message)
}

func (l *Logger) Success(origin string, message string) {
	l.Write("SUCCESS", origin, message)
}

func (l *Logger) Error(origin string, message string) {
	l.Write("ERROR", origin, message)
}

func (l *Logger) Blocked(origin string, message string) {
	l.Write("BLOCKED", origin, message)
}
