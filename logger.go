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
	maxPoolBufSize = 2048
	// base size when creating a buffer for the pool
	basePoolBufferSize = 512
	// waiting time for the automatic Flush() call
	flushTime = time.Millisecond * 250
)

var (
	reset   = []byte("\033[0m")
	red     = []byte("\033[31m")
	green   = []byte("\033[32m")
	yellow  = []byte("\033[33m")
	blue    = []byte("\033[34m")
	magenta = []byte("\033[35m")
	cyan    = []byte("\033[36m")
	none    = []byte("")
)

const (
	LevelDebug = "DEBU"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERRO"
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
	// logger level
	Level int `env:"LOG_LEVEL"`
	// start buffered output to minimize count of syscall, buff size - 4096
	BufferedOutput bool `env:"LOG_BUFFERED"`
}

type colorOptions struct {
	TimeColor  []byte
	KeyColor   []byte
	ValueColor []byte
}

func newColorOptions(timeColor, keyColor, valueColor []byte) *colorOptions {
	return &colorOptions{
		TimeColor:  timeColor,
		KeyColor:   keyColor,
		ValueColor: valueColor,
	}
}

type ColorizedHandler struct {
	colorOpts *colorOptions

	// holds the state common to all clones of the handler (writer, mutex, flags).
	shared *shared

	level slog.Level
	// groupPrefix stores the accumulated group name (e.g., "http.server.")
	// to flatten nested groups into dot-notation keys.
	groupPrefix string

	// precomputed stores already formatted attributes from WithAttrs()
	precomputed string
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

func NewTextHandler(w io.Writer, cfg *Config) *ColorizedHandler {
	if w == nil {
		w = os.Stderr
	}
	if cfg == nil {
		cfg = &Config{Level: 0, BufferedOutput: false}
	}

	shared := &shared{
		mu:     &sync.Mutex{},
		w:      w,
		done:   make(chan struct{}),
		closed: atomic.Bool{},
	}

	h := &ColorizedHandler{
		colorOpts: newColorOptions(blue, magenta, none),
		shared:    shared,
		level:     slog.Level(cfg.Level),
	}

	if cfg.BufferedOutput {
		bw := bufio.NewWriterSize(w, writerBufSize)
		h.shared.bw = bw
		// Start a background routine to periodically flush the buffer.
		// This ensures logs appear even during low activity periods.
		// NOTE: The user MUST call Close() to stop this goroutine and prevent leaks.
		go h.flusher()
	}

	return h
}

// flusher periodically flushes the buffer to the writer.
// It stops when the done channel is closed.
func (h *ColorizedHandler) flusher() {
	ticker := time.NewTicker(flushTime)
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
func (h *ColorizedHandler) flushBuffer() {
	h.shared.mu.Lock()
	_ = h.shared.bw.Flush()
	h.shared.mu.Unlock()
}

var (
	ErrNothingToClose = errors.New("use of close() is supported only for buffered logging")
	ErrAlreadyClosed  = errors.New("logger buffer already closed")
)

// Close signals the flusher to stop, marks the handler as closed using an atomic flag and flush buffer.
// Closes buffered output only.
func (h *ColorizedHandler) Close(_ context.Context) error {
	// todo close write to io.Writer, not only bufio.Writer
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

func (h *ColorizedHandler) Enabled(_ context.Context, level slog.Level) bool {
	if h.shared.closed.Load() {
		return false
	}
	return level >= h.level
}

func (h *ColorizedHandler) Handle(ctx context.Context, record slog.Record) (err error) {
	if h.shared.closed.Load() {
		return nil
	}

	// Check the ctx for slog.Args
	if ctx != nil {
		if val, ok := ctx.Value(AttrsKey).([]slog.Attr); ok {
			record.AddAttrs(val...)
		}
	}

	// Acquire a buffer from the pool to minimize garbage collection pressure.
	pBuf := bufPool.Get().(*[]byte)
	// Reset buffer length but keep capacity.
	buf := (*pBuf)[:0]

	buf = h.buildLog(buf, record)

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
	if cap(buf) <= maxPoolBufSize {
		*pBuf = buf
		bufPool.Put(pBuf)
	}

	return err
}

// WithGroup  returns a new slog.Handler that adds the passed group to all attrs.
func (h *ColorizedHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := h.clone()

	h2.groupPrefix = h2.groupPrefix + name + "."

	return h2
}

// WithAttrs returns a new slog.Handler with the given attributes appended.
func (h *ColorizedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	// Temporary buffer for parsing attributes.
	buf := make([]byte, 0, 512)

	// Existing precomputed attributes must come first.
	buf = append(buf, h.precomputed...)

	// Prepare the current group prefix for these specific attributes.
	var groupBuf [128]byte
	pref := groupBuf[:0]

	// Add group from WithGroup()
	if len(h.groupPrefix) > 0 {
		pref = append(pref, h.groupPrefix...)
	}

	for _, attr := range attrs {
		buf = h.appendAttr(buf, pref, attr)
	}

	h2 := h.clone()

	h2.precomputed = string(buf)
	return h2
}

// clone create new Handler with common state, groupPrefix and precomputed data.
func (h *ColorizedHandler) clone() *ColorizedHandler {
	return &ColorizedHandler{
		colorOpts:   h.colorOpts,
		shared:      h.shared,
		level:       h.level,
		groupPrefix: h.groupPrefix,
		precomputed: h.precomputed,
	}
}

//func (h *ColorizedHandler) WithGroup(name string) slog.handler {
//	if name == "" {
//		return h
//	}
//
//	h2 := h.clone()
//
//	// Pre-allocate to avoid multiple re-allocations during append
//	h2.groupPrefix = slices.Grow(h2.groupPrefix, len(name)+1)
//
//	h2.groupPrefix = append(h2.groupPrefix, name...)
//	h2.groupPrefix = append(h2.groupPrefix, '.')
//
//	return h2
//}

//func (h *ColorizedHandler) WithAttrs(attrs []slog.Attr) slog.handler {
//	if len(attrs) == 0 {
//		return h
//	}
//	h2 := h.clone()
//
//	// Calculate estimated size more precisely to reduce allocations
//	var estimatedSize int
//	for _, v := range attrs {
//		estimatedSize += len(v.Key) + 64
//	}
//
//	h2.precomputed = slices.Grow(h2.precomputed, estimatedSize)
//
//	// stack allocated buffer for group prefix
//	var groupBuf [128]byte
//	pref := groupBuf[:0]
//
//	//  add groupPrefix for attrs
//	if len(h2.groupPrefix) > 0 {
//		pref = append(pref, h2.groupPrefix...)
//	}
//
//	//pref := h2.groupPrefix
//	for _, attr := range attrs {
//		h2.precomputed = h.appendAttr(h2.precomputed, pref, attr)
//	}
//
//	return h2
//}

//func (h *ColorizedHandler) clone() *ColorizedHandler {
//	return &ColorizedHandler{
//		colorOpts: h.colorOpts,
//		mu:        h.mu,
//		w:         h.w,
//		level:     h.level,
//		// slices.Clip is CRITICAL here. It removes unused capacity.
//		// This forces the next 'append' in the child (h2) to allocate a NEW array,
//		// preventing it from overwriting the parent's (h) future data if they shared the same backing array.
//		groupPrefix: slices.Clip(h.groupPrefix),
//		precomputed: slices.Clip(h.precomputed),
//	}
//}
