package logger

import (
	"log/slog"
	"strings"
)

func levelColor(l slog.Level) []byte {
	switch l {
	case slog.LevelDebug:
		return blue
	case slog.LevelInfo:
		return green
	case slog.LevelWarn:
		return yellow
	case slog.LevelError:
		return red
	default:
		return none
	}
}

func parseLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func levelBytes(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return LevelDebug
	case slog.LevelInfo:
		return LevelInfo
	case slog.LevelWarn:
		return LevelWarn
	case slog.LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}
