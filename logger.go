package messagestream

import (
	"io"
	"log/slog"
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func SetLogger(l *slog.Logger) {
	logger = l
}
