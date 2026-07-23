// Package logger configures structured (slog) + Gin logging with file rotation,
// mirroring the HR service's logger so log handling is consistent across the
// cluster.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup wires slog and Gin to write JSON to both stdout (visible in `docker
// logs`) and a rotating file under logDir.
func Setup(logDir, filename string) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		slog.Error("Failed to create log directory", "error", err)
		os.Exit(1)
	}

	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, filename),
		MaxSize:    10, // MB
		MaxBackups: 5,
		MaxAge:     28, // days
		Compress:   true,
	}

	mw := io.MultiWriter(os.Stdout, rotator)
	gin.DefaultWriter = mw

	handler := slog.NewJSONHandler(mw, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	slog.Info("Logger initialized", "path", filepath.Join(logDir, filename))
}
