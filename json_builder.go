package logger

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"
	"unicode/utf8"
)

type jsonBuilder struct {
}

func NewJsonHandler(w io.Writer, cfg *Config) *Handler {
	if w == nil {
		w = os.Stderr
	}

	if cfg == nil {
		cfg = &Config{Level: 0, BufferedOutput: false}
	}

	handler := newHandler(w, slog.Level(cfg.Level), &jsonBuilder{})

	if cfg.BufferedOutput {
		handler.shared.bw = bufio.NewWriterSize(w, writerBufSize)
		// Start a background routine to periodically flush the buffer.
		// This ensures logs appear even during low activity periods.
		go handler.flusher()
	}

	return handler
}

func (b *jsonBuilder) buildLog(buf []byte, record slog.Record, precomputedAttrs string, groupPrefix string) []byte {
	buf = append(buf, `{"time":"`...)
	buf = record.Time.AppendFormat(buf, time.DateTime)
	buf = append(buf, `","level":"`...)
	buf = append(buf, levelBytes(record.Level)...)
	buf = append(buf, `","msg":"`...) // todo if no message
	buf = append(buf, record.Message...)
	buf = append(buf, '"')

	if record.NumAttrs() > 0 || precomputedAttrs != "" {
		buf = append(buf, ',')
		if groupPrefix != "" {
			buf = append(buf, groupPrefix...)
		}

		if record.NumAttrs() > 0 {

			if precomputedAttrs != "" {
				buf = append(buf, precomputedAttrs...)
				buf = append(buf, ',')
			}

			var isFirst = true
			record.Attrs(func(attr slog.Attr) bool {
				//attr.Value = attr.Value.Resolve()
				//if attr.Equal(slog.Attr{}) {
				//	return true
				//}

				if !isFirst {
					buf = append(buf, ',')
				} else {
					isFirst = false
				}
				buf = b.appendAttr(buf, nil, attr)
				return true
			})
		} else {
			buf = append(buf, precomputedAttrs...)
		}

		if groupPrefix != "" {
			buf = append(buf, '}')
		}
	}

	buf = append(buf, '}', '\n')

	return buf
}

func (b *jsonBuilder) appendAttr(buf []byte, _ []byte, attr slog.Attr) []byte {
	//attr.Value = attr.Value.Resolve()

	if attr.Equal(slog.Attr{}) {
		return buf
	}

	// Handle nested groups by recursion.
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()

		// If no attrs in group - slog.Group("group",).
		if len(group) == 0 {
			return buf
		}

		if attr.Key != "" {
			buf = append(buf, '"')
			//buf = append(buf, attr.Key...)
			buf = appendEscapedJSONString(buf, attr.Key)
			buf = append(buf, `":{`...)
		}

		var isFirst = true
		for _, v := range group {
			if v.Equal(slog.Attr{}) {
				continue
			}

			if !isFirst {
				buf = append(buf, ',')
			} else {
				isFirst = false
			}

			buf = b.appendAttr(buf, nil, v)
		}

		if attr.Key != "" {
			buf = append(buf, '}')
		}

		return buf
	}

	// Write key.
	buf = append(buf, '"')
	if attr.Key == "" {
		buf = append(buf, "!EMPTY_KEY"...)
	} else {
		buf = appendEscapedJSONString(buf, attr.Key)

	}
	buf = append(buf, `":`...)

	// Write value.
	buf = b.writeValue(buf, attr.Value)

	return buf
}

func (b *jsonBuilder) writeValue(buf []byte, value slog.Value) []byte {
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
		buf = strconv.AppendInt(buf, value.Duration().Nanoseconds(), 10)
	case slog.KindTime:
		buf = append(buf, '"')
		buf = value.Time().AppendFormat(buf, time.DateTime)
		buf = append(buf, '"')
	case slog.KindAny:
		if err, ok := value.Any().(error); ok {
			buf = append(buf, err.Error()...)
			return buf
		}
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

func (b *jsonBuilder) appendString(buf []byte, val string) []byte {
	buf = append(buf, '"')
	if val == "" {
		buf = append(buf, "!EMPTY_VALUE"...)
	} else {
		buf = appendEscapedJSONString(buf, val)
	}
	buf = append(buf, '"')

	return buf
}

func (b *jsonBuilder) precomputeAttrs(buf []byte, _ string, attrs []slog.Attr) []byte {
	var attrsCount = len(attrs) - 1

	for i, attr := range attrs {
		buf = b.appendAttr(buf, nil, attr)

		if attrsCount != i {
			buf = append(buf, ',')
		}
	}

	return buf
}

func (b *jsonBuilder) groupPrefix(oldPrefix string, newPrefix string) string {
	return oldPrefix + `"` + newPrefix + `":{`
}

// From stdlib.

const hex = "0123456789abcdef"

func appendEscapedJSONString(buf []byte, s string) []byte {
	char := func(b byte) { buf = append(buf, b) }
	str := func(s string) { buf = append(buf, s...) }

	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if safeSet[b] {
				i++
				continue
			}
			if start < i {
				str(s[start:i])
			}
			char('\\')
			switch b {
			case '\\', '"':
				char(b)
			case '\n':
				char('n')
			case '\r':
				char('r')
			case '\t':
				char('t')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				str(`u00`)
				char(hex[b>>4])
				char(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				str(s[start:i])
			}
			str(`\ufffd`)
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				str(s[start:i])
			}
			str(`\u202`)
			char(hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		str(s[start:])
	}
	return buf
}
