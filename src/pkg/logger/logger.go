// Package logger configures structured (slog) + Gin logging with file rotation,
// mirroring the HR service's logger so log handling is consistent across the
// cluster.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup wires slog and Gin to write JSON to both stdout (visible in `docker
// logs`) and a rotating file under logDir. It returns an error rather than
// exiting so the Windows service handler can report a failed start to the SCM;
// under the SCM stdout goes nowhere, which makes the rotating file the only
// record of what the gateway did.
func Setup(logDir, filename string) error {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log directory %s: %w", logDir, err)
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
	return nil
}
