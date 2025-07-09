package logger

import (
	"log/slog"
	"os"
)

// L is the package level logger used across the application.
var L = slog.New(slog.NewTextHandler(os.Stdout, nil))

// Set replaces the default logger with the provided one.
func Set(l *slog.Logger) {
	if l != nil {
		L = l
	}
}
