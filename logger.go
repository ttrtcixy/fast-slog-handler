package logger

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"sync"
	"time"
)

const (
	writerBufSize      = 4096
	maxPoolBufSize     = 2048
	basePoolBufferSize = 512
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

var (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// bufPool uses a pointer to a slice (*[]byte) to minimize overhead.
// Putting a raw []byte (header) into an interface{} causes an allocation.
// Storing a pointer fits better within the interface word size.
var bufPool = sync.Pool{
	New: func() any {
		// Use a pointer to slice to avoid allocation when putting back to pool
		b := make([]byte, 0, basePoolBufferSize)
		return &b
	},
}

type Config struct {
	Level string `env:"LOG_LEVEL"`
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

	// Use a shared mutex to protect the underlying io.Writer.
	mu *sync.Mutex
	w  *bufio.Writer

	level slog.Level

	// groupPrefix stores the accumulated group name (e.g., "http.server.")
	// to flatten nested groups into dot-notation keys.
	groupPrefix string

	// precomputed stores already formatted attributes from WithAttrs()
	// as a raw string to append it directly to the buffer, saving CPU cycles.
	precomputed string
}

func NewTextHandler(w io.Writer, cfg Config) *ColorizedHandler {
	h := &ColorizedHandler{
		colorOpts: newColorOptions(blue, magenta, none),
		mu:        &sync.Mutex{},
		w:         bufio.NewWriterSize(w, writerBufSize),
		level:     parseLevel(cfg.Level),
	}

	// Start a background routine to periodically flush the buffer.
	// This ensures logs appear even during low activity periods.
	go h.flusher()

	return h
}

// flusher periodically flushes the buffer to the underlying writer.
// NOTE: This goroutine runs until the application exits.
// If the handler is meant to be short-lived, a Close() method with a done channel is needed.
// todo Close()
func (h *ColorizedHandler) flusher() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		_ = h.w.Flush()
		h.mu.Unlock()
	}
}

// Sync ensures all buffered logs are written to the output.
// Should be called before application exit.
func (h *ColorizedHandler) Sync() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.w.Flush()
}

func (h *ColorizedHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *ColorizedHandler) Handle(_ context.Context, record slog.Record) error {
	// Acquire a buffer from the pool to minimize garbage collection pressure.
	pBuf := bufPool.Get().(*[]byte)
	// Reset buffer length but keep capacity.
	buf := (*pBuf)[:0]

	buf = h.buildLog(buf, record)

	h.mu.Lock()
	_, err := h.w.Write(buf)
	h.mu.Unlock()

	// Return buffer to pool only if it hasn't grown too large.
	// This prevents one huge log message from permanently keeping a large chunk of memory.
	if cap(buf) <= maxPoolBufSize {
		*pBuf = buf
		bufPool.Put(pBuf)
	}

	return err
}

func (h *ColorizedHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := h.clone()

	h2.groupPrefix = h2.groupPrefix + name + "."

	return h2
}

func (h *ColorizedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	// Temporary buffer for parsing attributes.
	buf := make([]byte, 0, 512) // todo test for alloc

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

func (h *ColorizedHandler) clone() *ColorizedHandler {
	return &ColorizedHandler{
		colorOpts:   h.colorOpts,
		mu:          h.mu,
		w:           h.w,
		level:       h.level,
		groupPrefix: h.groupPrefix,
		precomputed: h.precomputed,
	}
}

//func (h *ColorizedHandler) WithGroup(name string) slog.Handler {
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

//func (h *ColorizedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
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
