package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps the zap logger with additional functionality
type Logger struct {
	*zap.Logger
SugaredLogger *zap.SugaredLogger
}

// New creates a new logger instance
func New(logLevel string, logFormat string, logFilePath string) (*Logger, error) {
	// Parse log level
	var level zapcore.Level
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn", "warning":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey: "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create encoder
	var encoder zapcore.Encoder
	if logFormat == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Create core
	var cores []zapcore.Core

	// Add console output
	consoleCore := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
	cores = append(cores, consoleCore)

	// Add file output if path is specified
	if logFilePath != "" {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Create file writer with time-based rotation
		currentTime := time.Now().Format("2006-01-02")
		filePath := filepath.Join(logDir, fmt.Sprintf("%s-%s.log", filepath.Base(logFilePath), currentTime))
		
		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		fileCore := zapcore.NewCore(encoder, zapcore.AddSync(file), level)
		cores = append(cores, fileCore)
	}

	// Create logger with cores
	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{
		Logger:        logger,
		SugaredLogger: logger.Sugar(),
	}, nil
}

// NewFromConfig creates a logger from configuration
func NewFromConfig(level string, format string, filePath string) (*Logger, error) {
	return New(level, format, filePath)
}

// FatalOnError logs fatal error and exits
func (l *Logger) FatalOnError(err error) {
	if err != nil {
		l.Fatal("encountered error", zap.Error(err))
	}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}
