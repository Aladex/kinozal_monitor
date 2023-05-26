package logger

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// LogMessage represents a structured log message.
type LogMessage struct {
	Time       string            `json:"time"`
	ID         string            `json:"id,omitempty"`
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Package    string            `json:"package"`
	Additional map[string]string `json:"additional,omitempty"`
}

// Logger holds a log.Logger
type Logger struct {
	internalLogger *log.Logger
	PackageName    string
}

// New creates a new Logger.
func New(packageName string) *Logger {
	return &Logger{
		internalLogger: log.New(os.Stdout, "", 0),
		PackageName:    packageName,
	}
}

// Info logs an info message.
func (l *Logger) Info(id, message string, additional map[string]string) {
	l.log("INFO", id, message, additional)
}

// Error logs an error message.
func (l *Logger) Error(id, message string, additional map[string]string) {
	l.log("ERROR", id, message, additional)
}

func (l *Logger) log(level, id, message string, additional map[string]string) {
	logMessage := LogMessage{
		Time:       time.Now().Format(time.RFC3339Nano),
		ID:         id,
		Level:      level,
		Message:    message,
		Package:    l.PackageName,
		Additional: additional,
	}

	logMessageJSON, _ := json.Marshal(logMessage)
	l.internalLogger.Println(string(logMessageJSON))
}
