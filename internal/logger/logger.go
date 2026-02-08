package logger

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
)

var (
	global     *log.Logger
	globalOnce sync.Once
	globalFile *os.File
)

func Init(dir string) {
	globalOnce.Do(func() {
		if dir == "" {
			global = log.Default()
			return
		}
		os.MkdirAll(dir, 0755)
		f, err := os.OpenFile(filepath.Join(dir, "otter.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			global = log.Default()
			return
		}
		globalFile = f
		global = log.NewWithOptions(f, log.Options{Level: log.DebugLevel, ReportTimestamp: true})
	})
}

func g() *log.Logger {
	if global == nil {
		return log.Default()
	}
	return global
}

func Debug(msg string, keyvals ...any) { g().Debug(msg, keyvals...) }
func Info(msg string, keyvals ...any)  { g().Info(msg, keyvals...) }
func Warn(msg string, keyvals ...any)  { g().Warn(msg, keyvals...) }
func Error(msg string, keyvals ...any) { g().Error(msg, keyvals...) }

type Logger interface {
	WriteJSON(filename string, data []byte) error
	Debug(msg string, keyvals ...any)
	Info(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
	Error(msg string, keyvals ...any)
}

type FileLogger struct {
	dir  string
	once sync.Once
}

func NewFileLogger(dir string) *FileLogger {
	return &FileLogger{dir: dir}
}

func (l *FileLogger) init() {
	l.once.Do(func() {
		os.MkdirAll(l.dir, 0755)
	})
}

func (l *FileLogger) WriteJSON(filename string, data []byte) error {
	l.init()
	return os.WriteFile(filepath.Join(l.dir, filename), data, 0644)
}

func (l *FileLogger) Debug(msg string, keyvals ...any) { Debug(msg, keyvals...) }
func (l *FileLogger) Info(msg string, keyvals ...any)  { Info(msg, keyvals...) }
func (l *FileLogger) Warn(msg string, keyvals ...any)  { Warn(msg, keyvals...) }
func (l *FileLogger) Error(msg string, keyvals ...any) { Error(msg, keyvals...) }

func SessionLogDir(sessionsDir, sessionID string) string {
	return filepath.Join(sessionsDir, sessionID)
}

type nopLogger struct{}

func Nop() Logger                                { return nopLogger{} }
func (nopLogger) WriteJSON(string, []byte) error { return nil }
func (nopLogger) Debug(string, ...any)           {}
func (nopLogger) Info(string, ...any)            {}
func (nopLogger) Warn(string, ...any)            {}
func (nopLogger) Error(string, ...any)           {}
