package logger

import (
	"context"
	"log/slog"
	"time"
)

type Logger struct {
	log slog.Handler
}

func NewLogger(h slog.Handler) *Logger {
	return &Logger{
		log: h,
	}
}
func (l *Logger) logAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if !l.log.Enabled(ctx, level) {
		return
	}

	var pc uintptr

	record := slog.NewRecord(time.Now(), level, msg, pc)
	record.AddAttrs(attrs...)
	_ = l.log.Handle(ctx, record)
}

func (l *Logger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *Logger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logAttrs(ctx, slog.LevelError, msg, attrs...)
}

func (l *Logger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

func (l *Logger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

func String(key string, value string) slog.Attr {
	return slog.String(key, value)
}

func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

func Duration(key string, value time.Duration) slog.Attr {
	return slog.Duration(key, value)
}

func Time(key string, value time.Time) slog.Attr {
	return slog.Time(key, value)
}

func Any(key string, value any) slog.Attr {
	return slog.Any(key, value)
}

func Float64(key string, value float64) slog.Attr {
	return slog.Float64(key, value)
}

func Group(key string, attrs ...slog.Attr) slog.Attr {
	return slog.GroupAttrs(key, attrs...)
}

func (l *Logger) With(attrs ...slog.Attr) *Logger {
	if len(attrs) == 0 {
		return l
	}
	return &Logger{log: l.log.WithAttrs(attrs)}
}

func (l *Logger) WithGroup(name string) *Logger {
	if name == "" {
		return l
	}

	return &Logger{log: l.log.WithGroup(name)}
}

func (l *Logger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.log.Enabled(ctx, level)
}
