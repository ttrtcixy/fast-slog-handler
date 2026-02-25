package logger

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// size of buffio.Writer
	writerBufSize = 4096
	// max size of pool buffer
	maxPoolBufSize = 4096
	// base size when creating a buffer for the pool
	basePoolBufferSize = 2048
	// waiting time for the automatic Flush() call
	flushTime = time.Millisecond * 250
)

const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

var (
	ErrNothingToClose = errors.New("use of close() is supported only for buffered logging")
	ErrAlreadyClosed  = errors.New("logger buffer already closed")
)

// bufPool uses a pointer to a slice (*[]byte) to minimize overhead.
var bufPool = sync.Pool{
	New: func() any {
		// Use a pointer to slice to avoid allocation when putting back to pool
		b := make([]byte, 0, basePoolBufferSize)
		return &b
	},
}

type Config struct {
	// logger level, default - slog.LevelError
	Level slog.Level
	// start buffered output to minimize count of syscall.
	BufferedOutput bool
	// if BufferedOutput == true, you can specify the buffer size; if WriteBuffSize == 0, a buffer of size 4096 will be allocated.
	WriteBuffSize int
	// if BufferedOutput == true, you can specify the time after which the buffer will be cleared automatically, if FlushInterval == 0 clearing will occur every 250ms.
	FlushInterval time.Duration
	//// buffer size stored in the pool, default - 2048.
	//BaseBufPoolSize int
	// the maximum size to which the buffer in the pool can be expanded; after exceeding this size, the buffer is cleared by the garbage collector, default - 4096.
	MaxBufPoolSize int
}

// shared contains resources that must be synchronized across all handler clones.
type shared struct {
	// protects the underlying writers (bw and w).
	mu *sync.Mutex

	// buffered writer (can be nil if buffering is disabled).
	bw *bufio.Writer
	// underlying writer.
	w io.Writer

	// used to signal the flusher goroutine to stop.
	done chan struct{}
	// closed indicates whether the handler has been closed.
	closed atomic.Bool
}

type builderConstraint[B any] interface {
	jsonBuilder | colorizedTextBuilder
	buildLog(ctx context.Context, buf []byte, record slog.Record) []byte
	precomputeAttrs(attrs []slog.Attr) B
	groupPrefix(newPrefix string) B
}

type Handler[B builderConstraint[B]] struct {
	// holds the state common to all clones of the handler (writer, mutex, flags).
	shared *shared

	level slog.Level

	// builder implements the log formatting logic (text, json, etc.) abstracting it from the handler control flow.
	builder B

	// max pool buf size
	maxBufPoolSize int
}

// Close signals the flusher to stop, marks the handler as closed using an atomic flag and flush buffer.
// Closes buffered output only.
func (h *Handler[B]) Close(_ context.Context) error {
	// If buffering was never create.
	if h.shared.bw == nil {
		return ErrNothingToClose
	}

	// If already closed, do nothing.
	if h.shared.closed.Swap(true) {
		return ErrAlreadyClosed
	}

	// Close the channel to signal the flusher goroutine to exit.
	close(h.shared.done)

	h.flushBuffer()
	return nil
}

// flusher periodically flushes the buffer to the writer.
// It stops when the done channel is closed.
func (h *Handler[B]) flusher(cfgFlushInterval time.Duration) {
	var ticker *time.Ticker

	if cfgFlushInterval <= 0 {
		ticker = time.NewTicker(flushTime)
	} else {
		ticker = time.NewTicker(cfgFlushInterval)
	}

	defer ticker.Stop()

	for {
		select {
		case <-h.shared.done:
			return
		case <-ticker.C:
			h.flushBuffer()
		}
	}
}

// flushBuffer writes any buffered data to the underlying writer.
func (h *Handler[B]) flushBuffer() {
	h.shared.mu.Lock()
	_ = h.shared.bw.Flush()
	h.shared.mu.Unlock()
}

func newHandler[B builderConstraint[B]](w io.Writer, cfg *Config, builder B) *Handler[B] {
	if w == nil {
		w = os.Stderr
	}

	if cfg == nil {
		cfg = &Config{Level: slog.LevelInfo, BufferedOutput: false}
	}

	shared := &shared{
		mu:     &sync.Mutex{},
		w:      w,
		done:   make(chan struct{}),
		closed: atomic.Bool{},
	}

	handler := &Handler[B]{
		shared:         shared,
		level:          cfg.Level,
		builder:        builder,
		maxBufPoolSize: maxPoolBufSize,
	}

	if cfg.MaxBufPoolSize > 0 {
		handler.maxBufPoolSize = cfg.MaxBufPoolSize
	}

	if cfg.BufferedOutput {
		if cfg.WriteBuffSize > 0 {
			handler.shared.bw = bufio.NewWriterSize(w, cfg.WriteBuffSize)
		} else {
			handler.shared.bw = bufio.NewWriterSize(w, writerBufSize)
		}
		// Start a background routine to periodically flush the buffer.
		// This ensures logs appear even during low activity periods.
		go handler.flusher(cfg.FlushInterval)
	}

	return handler
}

func (h *Handler[B]) Enabled(_ context.Context, level slog.Level) bool {
	if h.shared.closed.Load() {
		return false
	}
	return level >= h.level
}

func (h *Handler[B]) Handle(ctx context.Context, record slog.Record) (err error) {
	if h.shared.closed.Load() {
		return nil
	}

	// Acquire a buffer from the pool to minimize garbage collection pressure.
	pBuf := bufPool.Get().(*[]byte)
	// Reset buffer length but keep capacity.
	buf := (*pBuf)[:0]

	buf = h.builder.buildLog(ctx, buf, record)

	if !h.shared.closed.Load() {
		h.shared.mu.Lock()
		if h.shared.bw != nil {
			_, err = h.shared.bw.Write(buf)
		} else {
			_, err = h.shared.w.Write(buf)
		}
		h.shared.mu.Unlock()
	}

	// Return buffer to pool only if it hasn't grown too large.
	// This prevents one huge handler message from permanently keeping a large chunk of memory.
	if cap(buf) <= h.maxBufPoolSize {
		*pBuf = buf
		bufPool.Put(pBuf)
	}

	return err
}

// WithGroup  returns a new slog.Handler that adds the passed group to all attrs.
func (h *Handler[B]) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := h.clone()
	h2.builder = h.builder.groupPrefix(name)

	return h2
}

// WithAttrs returns a new slog.Handler with the given attributes appended.
func (h *Handler[B]) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	h2 := h.clone()

	h2.builder = h.builder.precomputeAttrs(attrs)

	return h2
}

// clone create new Handler with common state, groupPrefix and precomputed data.
func (h *Handler[B]) clone() *Handler[B] {
	return &Handler[B]{
		shared:         h.shared,
		level:          h.level,
		builder:        h.builder,
		maxBufPoolSize: h.maxBufPoolSize,
	}
}

type loggerCtxKey struct {
}

var attrsKey = loggerCtxKey{}

func (h *Handler[builderT]) AppendAttrsToCtx(ctx context.Context, attrs ...slog.Attr) context.Context {
	return AppendAttrsToCtx(ctx, attrs...)
}

// AppendAttrsToCtx add []slog.Attr to ctx with attrsKey, if the ctx already contains arguments, add them to the existing ones.
// Attributes will be added at the beginning, outside of groups created using WithGroup.
func AppendAttrsToCtx(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}

	val, ok := ctx.Value(attrsKey).([]slog.Attr)
	if ok { // If attrs in ctx.
		if len(val) != 0 {
			attrs = append(val[:len(val):len(val)], attrs...)
		}
	}

	return context.WithValue(ctx, attrsKey, attrs)
}
