package logger

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"
)

//var (
//	reset   = []byte("\033[0m")
//	red     = []byte("\033[31m")
//	green   = []byte("\033[32m")
//	yellow  = []byte("\033[33m")
//	blue    = []byte("\033[34m")
//	magenta = []byte("\033[35m")
//	cyan    = []byte("\033[36m")
//	none    = []byte("")
//)

const (
	reset  = "\u001b[0m"
	faint  = "\u001b[2m"
	red    = "\u001b[91m"
	green  = "\u001b[92m"
	yellow = "\u001b[93m"
	blue   = "\u001b[94m"
)

type colorizedTextBuilder struct {
	//colorOpts *colorOptions
}

func NewTextHandler(w io.Writer, cfg *Config) *Handler {
	if w == nil {
		w = os.Stderr
	}

	if cfg == nil {
		cfg = &Config{Level: 0, BufferedOutput: false}
	}

	textBuilder := &colorizedTextBuilder{
		//colorOpts: newColorOptions(faint, faint),
	}

	handler := newHandler(w, slog.Level(cfg.Level), textBuilder)

	if cfg.BufferedOutput {
		handler.shared.bw = bufio.NewWriterSize(w, writerBufSize)
		// Start a background routine to periodically flush the buffer.
		// This ensures logs appear even during low activity periods.
		go handler.flusher()
	}

	return handler
}

func (b *colorizedTextBuilder) buildLog(
	buf []byte,
	record slog.Record,
	precomputedAttrs string,
	groupPrefix string,
) []byte {
	// Time
	buf = append(buf, faint...) // color
	buf = record.Time.AppendFormat(buf, time.Stamp)
	buf = append(buf, reset...) // color
	buf = append(buf, ' ')

	// Level
	buf = append(buf, levelColor(record.Level)...) // color
	buf = append(buf, levelBytes(record.Level)[:4]...)
	buf = append(buf, reset...) // color
	buf = append(buf, ' ')

	// Message // todo if no message
	buf = append(buf, record.Message...)

	// Append precomputed attributes (from WithAttrs)
	if len(precomputedAttrs) > 0 {
		buf = append(buf, precomputedAttrs...)
	}
	// Process dynamic attributes (attached to this specific record)
	if record.NumAttrs() > 0 {
		// Stack-allocated buffer for group prefix to avoid allocs
		var groupBuf [128]byte
		pref := groupBuf[:0]

		// Add group from WithGroup()
		if len(groupPrefix) > 0 {
			pref = append(pref, groupPrefix...)
		}

		record.Attrs(func(attr slog.Attr) bool {
			buf = b.appendAttr(buf, pref, attr)
			return true
		})
	}

	buf = append(buf, '\n')
	return buf
}

func (b *colorizedTextBuilder) appendAttr(buf []byte, groupPrefix []byte, attr slog.Attr) []byte {
	attr.Value = attr.Value.Resolve()

	if attr.Equal(slog.Attr{}) {
		return buf
	}

	// Handle nested groups by recursion: flattening keys to "prefix.key"
	if attr.Value.Kind() == slog.KindGroup {
		if attr.Key != "" {
			groupPrefix = append(groupPrefix, attr.Key...)
			groupPrefix = append(groupPrefix, '.')
		}

		for _, v := range attr.Value.Group() {
			buf = b.appendAttr(buf, groupPrefix, v)
		}
		return buf
	}

	buf = append(buf, ' ')
	buf = append(buf, faint...) // color

	if len(groupPrefix) > 0 {
		buf = append(buf, groupPrefix...)
	}

	if attr.Key == "" {
		attr.Key = "!EMPTY_KEY"
	}
	buf = append(buf, attr.Key...)
	buf = append(buf, '=')
	buf = append(buf, reset...) // color

	buf = b.writeValue(buf, attr.Value)

	return buf
}

func (b *colorizedTextBuilder) writeValue(buf []byte, value slog.Value) []byte {
	switch value.Kind() {
	case slog.KindString:
		buf = b.appendString(buf, value.String())
	case slog.KindInt64:
		buf = strconv.AppendInt(buf, value.Int64(), 10)
	case slog.KindUint64:
		buf = strconv.AppendUint(buf, value.Uint64(), 10)
	case slog.KindFloat64:
		buf = strconv.AppendFloat(buf, value.Float64(), 'f', -1, 64)
	case slog.KindBool:
		if value.Bool() {
			buf = append(buf, "true"...)
		} else {
			buf = append(buf, "false"...)
		}
	case slog.KindDuration:
		buf = append(buf, value.Duration().String()...)
	case slog.KindTime:
		buf = value.Time().AppendFormat(buf, time.DateTime)
	case slog.KindAny:
		b, err := json.Marshal(value.Any())
		if err != nil {
			buf = append(buf, "!ERR_MARSHAL"...)
		} else {
			buf = append(buf, b...)
		}
	default:
		buf = append(buf, "!UNHANDLED"...)
	}
	return buf
}

func (b *colorizedTextBuilder) precomputeAttrs(buf []byte, groupPrefix string, attrs []slog.Attr) []byte {
	// Prepare the current group prefix for these specific attributes.
	var groupBuf [128]byte
	pref := groupBuf[:0]

	// Add group from WithGroup()
	if len(groupPrefix) > 0 {
		pref = append(pref, groupPrefix...)
	}

	for _, attr := range attrs {
		buf = b.appendAttr(buf, pref, attr)
	}

	return buf
}

func (b *colorizedTextBuilder) groupPrefix(oldPrefix string, newPrefix string) string {
	return oldPrefix + newPrefix + "."
}

func (b *colorizedTextBuilder) appendString(buf []byte, val string) []byte {
	if val == "" {
		buf = append(buf, "!EMPTY_VALUE"...)
	} else {
		if needsQuoting(val) {
			buf = strconv.AppendQuote(buf, val)
		} else {
			buf = append(buf, val...)
		}
	}
	return buf
}

func needsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			if b != '\\' && (b == ' ' || b == '=' || !safeSet[b]) {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
}
