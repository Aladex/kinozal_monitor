package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogFormat represents the output format for logs
type LogFormat int

const (
	FormatJSON LogFormat = iota
	FormatText
	FormatStructured
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Gray   = "\033[37m"
	Green  = "\033[32m"
)

// LogMessage represents a structured log message
type LogMessage struct {
	Time       string                 `json:"time"`
	Level      string                 `json:"level"`
	Package    string                 `json:"package"`
	Message    string                 `json:"message"`
	ID         string                 `json:"id,omitempty"`
	Additional map[string]interface{} `json:"additional,omitempty"`
	Caller     string                 `json:"caller,omitempty"`
}

// Config holds the logger configuration
type Config struct {
	Level        LogLevel
	Format       LogFormat
	Output       io.Writer
	EnableColor  bool
	EnableCaller bool
	CallerSkip   int
	TimeFormat   string
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:        INFO,
		Format:       FormatText,
		Output:       os.Stdout,
		EnableColor:  true,
		EnableCaller: false,
		CallerSkip:   3,
		TimeFormat:   "2006-01-02 15:04:05",
	}
}

// Logger holds the logger configuration and internal logger
type Logger struct {
	config         *Config
	internalLogger *log.Logger
	packageName    string
}

// Global default logger
var defaultLogger *Logger

func init() {
	config := DefaultConfig()

	// Parse environment variables for configuration
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch strings.ToUpper(level) {
		case "DEBUG":
			config.Level = DEBUG
		case "INFO":
			config.Level = INFO
		case "WARN":
			config.Level = WARN
		case "ERROR":
			config.Level = ERROR
		case "FATAL":
			config.Level = FATAL
		}
	}

	if format := os.Getenv("LOG_FORMAT"); format != "" {
		switch strings.ToLower(format) {
		case "json":
			config.Format = FormatJSON
		case "text":
			config.Format = FormatText
		case "structured":
			config.Format = FormatStructured
		}
	}

	if os.Getenv("LOG_COLOR") == "false" {
		config.EnableColor = false
	}

	if os.Getenv("LOG_CALLER") == "true" {
		config.EnableCaller = true
	}

	defaultLogger = NewWithConfig("default", config)
}

// New creates a new Logger with default configuration
func New(packageName string) *Logger {
	return NewWithConfig(packageName, DefaultConfig())
}

// NewWithConfig creates a new Logger with custom configuration
func NewWithConfig(packageName string, config *Config) *Logger {
	return &Logger{
		config:         config,
		internalLogger: log.New(config.Output, "", 0),
		packageName:    packageName,
	}
}

// GetDefault returns the default global logger
func GetDefault() *Logger {
	return defaultLogger
}

// SetDefault sets the default global logger
func SetDefault(logger *Logger) {
	defaultLogger = logger
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	l.log(DEBUG, "", message, mergeFields(fields...))
}

// DebugWithID logs a debug message with an ID
func (l *Logger) DebugWithID(id, message string, fields ...map[string]interface{}) {
	l.log(DEBUG, id, message, mergeFields(fields...))
}

// Info logs an info message - backward compatible method
func (l *Logger) Info(id, message string, additional map[string]string) {
	fields := make(map[string]interface{})
	for k, v := range additional {
		fields[k] = v
	}
	l.log(INFO, id, message, fields)
}

// InfoNew logs an info message with new API
func (l *Logger) InfoNew(message string, fields ...map[string]interface{}) {
	l.log(INFO, "", message, mergeFields(fields...))
}

// InfoWithID logs an info message with an ID
func (l *Logger) InfoWithID(id, message string, fields ...map[string]interface{}) {
	l.log(INFO, id, message, mergeFields(fields...))
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	l.log(WARN, "", message, mergeFields(fields...))
}

// WarnWithID logs a warning message with an ID
func (l *Logger) WarnWithID(id, message string, fields ...map[string]interface{}) {
	l.log(WARN, id, message, mergeFields(fields...))
}

// Error logs an error message - backward compatible method
func (l *Logger) Error(id, message string, additional map[string]string) {
	fields := make(map[string]interface{})
	for k, v := range additional {
		fields[k] = v
	}
	l.log(ERROR, id, message, fields)
}

// ErrorNew logs an error message with new API
func (l *Logger) ErrorNew(message string, fields ...map[string]interface{}) {
	l.log(ERROR, "", message, mergeFields(fields...))
}

// ErrorWithID logs an error message with an ID
func (l *Logger) ErrorWithID(id, message string, fields ...map[string]interface{}) {
	l.log(ERROR, id, message, mergeFields(fields...))
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	l.log(FATAL, "", message, mergeFields(fields...))
	os.Exit(1)
}

// FatalWithID logs a fatal message with an ID and exits
func (l *Logger) FatalWithID(id, message string, fields ...map[string]interface{}) {
	l.log(FATAL, id, message, mergeFields(fields...))
	os.Exit(1)
}

// Backward compatibility methods to support legacy method signatures
// InfoLegacy provides backward compatibility for the old Info method signature
func (l *Logger) InfoLegacy(id, message string, additional map[string]string) {
	fields := make(map[string]interface{})
	for k, v := range additional {
		fields[k] = v
	}
	l.InfoWithID(id, message, fields)
}

// ErrorLegacy provides backward compatibility for the old Error method signature
func (l *Logger) ErrorLegacy(id, message string, additional map[string]string) {
	fields := make(map[string]interface{})
	for k, v := range additional {
		fields[k] = v
	}
	l.ErrorWithID(id, message, fields)
}

// Legacy method signatures that match the old API exactly
// These methods override the new ones to maintain backward compatibility
func (l *Logger) InfoOld(id, message string, additional map[string]string) {
	l.InfoLegacy(id, message, additional)
}

func (l *Logger) ErrorOld(id, message string, additional map[string]string) {
	l.ErrorLegacy(id, message, additional)
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, id, message string, additional map[string]interface{}) {
	if level < l.config.Level {
		return
	}

	now := time.Now()
	caller := ""

	if l.config.EnableCaller {
		if pc, file, line, ok := runtime.Caller(l.config.CallerSkip); ok {
			if fn := runtime.FuncForPC(pc); fn != nil {
				caller = fmt.Sprintf("%s:%d", file, line)
			}
		}
	}

	logMessage := LogMessage{
		Time:       now.Format(l.config.TimeFormat),
		Level:      level.String(),
		Package:    l.packageName,
		Message:    message,
		ID:         id,
		Additional: additional,
		Caller:     caller,
	}

	var output string
	switch l.config.Format {
	case FormatJSON:
		output = l.formatJSON(logMessage)
	case FormatStructured:
		output = l.formatStructured(logMessage)
	default:
		output = l.formatText(logMessage)
	}

	l.internalLogger.Println(output)
}

// formatJSON formats the log message as JSON
func (l *Logger) formatJSON(msg LogMessage) string {
	data, _ := json.Marshal(msg)
	return string(data)
}

// formatText formats the log message as plain text
func (l *Logger) formatText(msg LogMessage) string {
	var sb strings.Builder

	levelColor := ""
	resetColor := ""

	if l.config.EnableColor {
		switch msg.Level {
		case "DEBUG":
			levelColor = Gray
		case "INFO":
			levelColor = Green
		case "WARN":
			levelColor = Yellow
		case "ERROR":
			levelColor = Red
		case "FATAL":
			levelColor = Red
		}
		resetColor = Reset
	}

	sb.WriteString(fmt.Sprintf("%s[%s]%s %s", levelColor, msg.Level, resetColor, msg.Time))

	if msg.Package != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", msg.Package))
	}

	if msg.ID != "" {
		sb.WriteString(fmt.Sprintf(" {%s}", msg.ID))
	}

	sb.WriteString(fmt.Sprintf(" %s", msg.Message))

	if msg.Caller != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", msg.Caller))
	}

	if len(msg.Additional) > 0 {
		sb.WriteString(" |")
		for k, v := range msg.Additional {
			sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
	}

	return sb.String()
}

// formatStructured formats the log message in a structured format
func (l *Logger) formatStructured(msg LogMessage) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("time=%s level=%s", msg.Time, msg.Level))

	if msg.Package != "" {
		sb.WriteString(fmt.Sprintf(" package=%s", msg.Package))
	}

	if msg.ID != "" {
		sb.WriteString(fmt.Sprintf(" id=%s", msg.ID))
	}

	sb.WriteString(fmt.Sprintf(" msg=\"%s\"", msg.Message))

	if msg.Caller != "" {
		sb.WriteString(fmt.Sprintf(" caller=%s", msg.Caller))
	}

	for k, v := range msg.Additional {
		sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
	}

	return sb.String()
}

// mergeFields merges multiple field maps into one
func mergeFields(fields ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, field := range fields {
		for k, v := range field {
			result[k] = v
		}
	}
	return result
}

// Global convenience functions using the default logger
func DebugGlobal(message string, fields ...map[string]interface{}) {
	defaultLogger.Debug(message, fields...)
}

func InfoGlobal(message string, fields ...map[string]interface{}) {
	defaultLogger.InfoNew(message, fields...)
}

func WarnGlobal(message string, fields ...map[string]interface{}) {
	defaultLogger.Warn(message, fields...)
}

func ErrorGlobal(message string, fields ...map[string]interface{}) {
	defaultLogger.ErrorNew(message, fields...)
}

func FatalGlobal(message string, fields ...map[string]interface{}) {
	defaultLogger.Fatal(message, fields...)
}
