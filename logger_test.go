package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

var (
	attr1 = slog.Group("group", slog.String("key", "val"))
	attr2 = slog.Group("group", slog.String("key", "val"), slog.Group("under_group", slog.String("qwe", "tte")))
)

var file, _ = os.OpenFile("t.txt", os.O_RDWR, 0000)

func BenchmarkLoggerJsonHandlerBuffered(b *testing.B) {
	handler := NewJsonHandler(io.Discard, &Config{Level: slog.LevelInfo, BufferedOutput: true})
	logger := slog.New(handler)

	ctx := context.Background()
	ctx = handler.AppendAttrsToCtx(ctx, slog.String("ctx", "attr"))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

			logger.LogAttrs(ctx, slog.LevelInfo, "msg",
				slog.String("user_id", "user_99"),
				slog.Int("amount", 500),
				slog.Duration("latency", 15*time.Millisecond),
				slog.Bool("retry", false),
				attr1,
				attr2,
			)
		}
	})
}

func BenchmarkLoggerJsonHandler(b *testing.B) {
	handler := NewJsonHandler(io.Discard, &Config{Level: slog.LevelInfo, BufferedOutput: false})
	logger := slog.New(handler)

	ctx := context.Background()
	ctx = handler.AppendAttrsToCtx(ctx, slog.String("ctx", "attr"))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

			logger.LogAttrs(ctx, slog.LevelInfo, "msg",
				slog.String("user_id", "user_99"),
				slog.Int("amount", 500),
				slog.Duration("latency", 15*time.Millisecond),
				slog.Bool("retry", false),
				attr1,
				attr2,
			)
		}
	})
}

func BenchmarkLoggerTextBuffered(b *testing.B) {
	handler := NewTextHandler(io.Discard, &Config{Level: slog.LevelInfo, BufferedOutput: true})
	logger := slog.New(handler)

	ctx := context.Background()
	ctx = handler.AppendAttrsToCtx(ctx, slog.String("ctx", "attr"))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

			logger.LogAttrs(ctx, slog.LevelInfo, "msg",
				slog.String("user_id", "user_99"),
				slog.Int("amount", 500),
				slog.Duration("latency", 15*time.Millisecond),
				slog.Bool("retry", false),
				attr1,
				attr2,
			)
		}
	})
}

func BenchmarkLoggerText(b *testing.B) {
	handler := NewTextHandler(io.Discard, &Config{Level: slog.LevelInfo, BufferedOutput: false})
	logger := slog.New(handler)

	ctx := context.Background()
	ctx = handler.AppendAttrsToCtx(ctx, slog.String("ctx", "attr"))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

			logger.LogAttrs(ctx, slog.LevelInfo, "msg",
				slog.String("user_id", "user_99"),
				slog.Int("amount", 500),
				slog.Duration("latency", 15*time.Millisecond),
				slog.Bool("retry", false),
				attr1,
				attr2,
			)
		}
	})
}
