package logger

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

var (
	pattr1 = Group("group", String("key", "val"))
	pattr2 = Group("group", String("key", "val"), Group("under_group", String("qwe", "tte")))
)

var (
	attr1 = slog.Group("group", slog.String("key", "val"))
	attr2 = slog.Group("group", slog.String("key", "val"), slog.Group("under_group", slog.String("qwe", "tte")))
)

func BenchmarkLoggerSlogColorHandler(b *testing.B) {
	logger := NewLogger(NewTextHandler(io.Discard, Config{Level: LevelDebug}))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//logger := logger.WithGroup("test_g")
			//logger = logger.With(String("qwe", "tte"))

			logger.Info(nil, "msg",
				String("user_id", "user_99"),
				Int("amount", 500),
				Duration("latency", 15*time.Millisecond),
				Bool("retry", false),
				pattr1,
				pattr2,
			)
		}
	})
}

func BenchmarkLoggerSlog(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//logger := logger.WithGroup("test_g")
			//logger = logger.With(slog.String("qwe", "tte"))

			logger.LogAttrs(nil, slog.LevelInfo, "msg",
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
