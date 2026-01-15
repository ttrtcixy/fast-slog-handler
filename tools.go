package logger

import (
	"context"
	"log/slog"
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

func ParseLevel(level int) string {
	switch slog.Level(level) {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return "INFO"
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

type loggerCtxKey struct {
}

var AttrsKey = loggerCtxKey{}

// AppendAttrsToCtx add []slog.Attr to ctx with AttrsKey, if the ctx already contains arguments, add them to the existing ones.
func AppendAttrsToCtx(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}

	val, ok := ctx.Value(AttrsKey).([]slog.Attr)
	if ok { // If attrs in ctx.
		if len(val) != 0 {
			attrs = append(val[:len(val):len(val)], attrs...)
		}
	}

	return context.WithValue(ctx, AttrsKey, attrs)
}
