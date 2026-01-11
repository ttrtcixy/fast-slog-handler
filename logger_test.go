package logger

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

var (
	attr1 = slog.Group("group", slog.String("key", "val"))
	attr2 = slog.Group("group", slog.String("key", "val"), slog.Group("under_group", slog.String("qwe", "tte")))
)

var file, _ = os.OpenFile("test.txt", os.O_RDWR, 0000)

// На тестах было замечено, что цветной вывод замедляет код в 2+ раза.

func BenchmarkLoggerSlogColorHandlerBuffered(b *testing.B) {
	logger := NewLogger(NewTextHandler(os.Stdout, &Config{Level: int(slog.LevelDebug), BufferedOutput: true}))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

			logger.logAttrs(nil, slog.LevelInfo, "msg",
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

func BenchmarkLoggerSlogColorHandler(b *testing.B) {
	logger := NewLogger(NewTextHandler(os.Stdout, &Config{Level: int(slog.LevelDebug)}))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

			logger.logAttrs(nil, slog.LevelInfo, "msg",
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

func BenchmarkLoggerSlog(b *testing.B) {
	//file, _ := os.OpenFile("test.txt", os.O_RDWR, 0000)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger := logger.WithGroup("test_g")
			logger = logger.With(slog.String("qwe", "tte"))

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
