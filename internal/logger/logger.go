package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// Logger provides thread-safe logging
type Logger struct {
	level      Level
	logger     *log.Logger
	mu         sync.Mutex
	projectDir string
	file       *os.File
}

var (
	instance *Logger
	once     sync.Once
)

// Init initializes the logger (Singleton pattern)
func Init(levelStr string, logFile string) error {
	var err error
	once.Do(func() {
		level := parseLevel(levelStr)

		var writer io.Writer = os.Stdout
		var file *os.File

		// If log file is specified, write to file
		if logFile != "" {
			// Create log directory if not exists
			logDir := filepath.Dir(logFile)
			if mkdirErr := os.MkdirAll(logDir, 0755); mkdirErr != nil {
				err = fmt.Errorf("create log directory: %w", mkdirErr)
				return
			}

			var openErr error
			file, openErr = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if openErr != nil {
				err = fmt.Errorf("open log file: %w", openErr)
				return
			}
			// Write to both stdout and file
			writer = io.MultiWriter(os.Stdout, file)
		}

		// Get project root directory by finding go.mod
		projectDir, _ := findProjectRoot()

		instance = &Logger{
			level:      level,
			logger:     log.New(writer, "", log.LstdFlags),
			projectDir: projectDir,
			file:       file,
		}
	})

	return err
}

// Get returns the logger instance
func Get() *Logger {
	if instance == nil {
		// If not initialized, use default config
		_ = Init("info", "")
	}
	return instance
}

func parseLevel(levelStr string) Level {
	switch levelStr {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// findProjectRoot finds the project root directory by looking for go.mod
func findProjectRoot() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree until we find go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func (l *Logger) log(level Level, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var prefix string
	switch level {
	case DEBUG:
		prefix = colorCyan + "[DEBUG]" + colorReset + " "
	case INFO:
		prefix = colorGreen + "[INFO]" + colorReset + " "
	case WARN:
		prefix = colorYellow + "[WARN]" + colorReset + " "
	case ERROR:
		prefix = colorRed + "[ERROR]" + colorReset + " "
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(3)
	fileInfo := ""
	if ok {
		// Calculate relative path from project root
		if l.projectDir != "" && strings.HasPrefix(file, l.projectDir) {
			relPath := strings.TrimPrefix(file, l.projectDir)
			relPath = strings.TrimPrefix(relPath, "/")
			fileInfo = fmt.Sprintf("%s:%d ", relPath, line)
		} else {
			// Fallback to filename only if project dir not found
			fileInfo = fmt.Sprintf("%s:%d ", filepath.Base(file), line)
		}
	}

	msg := fmt.Sprintf(format, v...)
	l.logger.Output(3, prefix+fileInfo+msg)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(WARN, format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// Package-level convenience functions
func Debug(format string, v ...interface{}) {
	Get().Debug(format, v...)
}

func Info(format string, v ...interface{}) {
	Get().Info(format, v...)
}

func Warn(format string, v ...interface{}) {
	Get().Warn(format, v...)
}

func Error(format string, v ...interface{}) {
	Get().Error(format, v...)
}

func Close() {
	if instance != nil {
		instance.Close()
	}
}
