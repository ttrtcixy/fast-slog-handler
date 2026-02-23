package logger

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/lmittmann/tint"
)

var (
	attr1 = slog.Group("group", slog.String("key", "val"))
	attr2 = slog.Group("group", slog.String("key", "val"), slog.Group("under_group", slog.String("qwe", "tte")))
)

var file, _ = os.OpenFile("test.txt", os.O_RDWR, 0000)

//func BenchmarkLoggerTextHandlerBuffered(b *testing.B) {
//	logger := slog.New(NewTextHandler(io.Discard, &Config{Level: int(slog.LevelDebug), BufferedOutput: true}))
//
//	b.ResetTimer()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			logger := logger.WithGroup("test_g")
//			logger = logger.With(slog.String("qwe", "tte"))
//
//			logger.LogAttrs(nil, slog.LevelInfo, "msg",
//				slog.String("user_id", "user_99"),
//				slog.Int("amount", 500),
//				slog.Duration("latency", 15*time.Millisecond),
//				slog.Bool("retry", false),
//				attr1,
//				attr2,
//			)
//		}
//	})
//}

// 35312 ns/op            3478 B/op         12 allocs/op
//func BenchmarkLoggerTextHandler(b *testing.B) {
//	logger := slog.New(NewTextHandler(file, &Config{Level: int(slog.LevelInfo)}))
//
//	b.ResetTimer()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			logger := logger.WithGroup("test_g")
//			logger = logger.With(slog.String("qwe", "tte"))
//
//			logger.LogAttrs(nil, slog.LevelInfo, "msg",
//				slog.String("user_id", "user_99"),
//				slog.Int("amount", 500),
//				slog.Duration("latency", 15*time.Millisecond),
//				slog.Bool("retry", false),
//				attr1,
//				attr2,
//			)
//		}
//	})
//}

// 37346 ns/op            3333 B/op         18 allocs/op
func BenchmarkLoggerTintHandler(b *testing.B) {
	logger := slog.New(tint.NewHandler(file, nil))

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

// buffered - 448.3 ns/op           851 B/op         10 allocs/op
// default - 360.2 ns/op           851 B/op         10 allocs/op
func BenchmarkLoggerJsonHandlerBuffered(b *testing.B) {
	logger := slog.New(NewJsonHandler(file, &Config{Level: slog.LevelInfo, BufferedOutput: false}))

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

//func BenchmarkLoggerJsonHandler(b *testing.B) {
//	logger := slog.New(NewJsonHandler(io.Discard, &Config{Level: int(slog.LevelDebug)}))
//
//	b.ResetTimer()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			logger := logger.WithGroup("test_g")
//			logger = logger.With(slog.String("qwe", "tte"))
//
//			logger.LogAttrs(nil, slog.LevelInfo, "msg",
//				slog.String("user_id", "user_99"),
//				slog.Int("amount", 500),
//				slog.Duration("latency", 15*time.Millisecond),
//				slog.Bool("retry", false),
//				attr1,
//				attr2,
//			)
//		}
//	})
//}
//
//func BenchmarkLoggerSlog(b *testing.B) {
//	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
//	b.ResetTimer()
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			logger := logger.WithGroup("test_g")
//			logger = logger.With(slog.String("qwe", "tte"))
//
//			logger.LogAttrs(nil, slog.LevelInfo, "msg",
//				slog.String("user_id", "user_99"),
//				slog.Int("amount", 500),
//				slog.Duration("latency", 15*time.Millisecond),
//				slog.Bool("retry", false),
//				attr1,
//				attr2,
//			)
//		}
//	})
//}
