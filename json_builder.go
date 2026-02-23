package logger

import (
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"
)

type jsonBuilder struct {
	// precomputed for jsonBuilder stores already formatted args from WithAttrs() and WithGroup()
	precomputed []byte
	//
	depth int
}

func NewJsonHandler(w io.Writer, cfg *Config) *Handler {
	return newHandler(w, cfg, &jsonBuilder{})
}

func (b *jsonBuilder) buildLog(buf []byte, record slog.Record, precomputed []byte, depth int) []byte {
	buf = append(buf, `{"time":"`...)
	buf = record.Time.AppendFormat(buf, time.DateTime)

	buf = append(buf, `","level":"`...)
	buf = append(buf, levelBytes(record.Level)...)

	buf = append(buf, `","msg":"`...)
	if record.Message == "" {
		buf = append(buf, `!EMPTY_MESSAGE`...)
	} else {
		buf = append(
			buf,
			record.Message...) // dangerous because it does not track whether there are invalid JSON characters in the line
	}
	buf = append(buf, '"')

	if len(precomputed) > 0 {
		buf = b.addComma(buf)
		buf = append(buf, precomputed...)
	}

	if record.NumAttrs() > 0 {
		record.Attrs(func(attr slog.Attr) bool {
			buf = b.appendAttr(buf, attr)
			return true
		})
	}

	for range depth {
		buf = append(buf, '}')
	}

	buf = append(buf, '}', '\n')

	return buf
}

func (b *jsonBuilder) appendAttr(buf []byte, attr slog.Attr) []byte {
	attr.Value = attr.Value.Resolve()

	if attr.Equal(slog.Attr{}) {
		return buf
	}

	// Handle nested groups by recursion.
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()

		// If no attrs in group - slog.Group("group").
		if len(group) == 0 {
			return buf
		}

		if attr.Key != "" {
			buf = b.addComma(buf)
			buf = append(buf, '"')
			//buf = append(buf, attr.Key...)
			buf = appendEscapedJSONString(buf, attr.Key)
			buf = append(buf, `":{`...)
		}

		for _, v := range group {
			buf = b.appendAttr(buf, v)
		}

		if attr.Key != "" {
			buf = append(buf, '}')
		}

		return buf
	}

	buf = b.addComma(buf)

	// Write key.
	buf = append(buf, '"')
	if attr.Key == "" {
		buf = append(buf, `!EMPTY_KEY`...)
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
		buf = value.Time().AppendFormat(buf, time.RFC3339Nano)
		buf = append(buf, '"')
	case slog.KindAny:
		if err, ok := value.Any().(error); ok {
			buf = b.appendString(buf, err.Error())
			return buf
		}
		b, err := json.Marshal(value.Any())
		if err != nil {
			buf = append(buf, `!ERR_MARSHAL`...)
		} else {
			buf = append(buf, b...)
		}
	default:
		buf = append(buf, `!UNHANDLED`...)
	}

	return buf
}

func (b *jsonBuilder) appendString(buf []byte, val string) []byte {
	buf = append(buf, '"')
	if val == "" {
		buf = append(buf, `!EMPTY_VALUE`...)
	} else {
		buf = appendEscapedJSONString(buf, val)
	}
	buf = append(buf, '"')

	return buf
}

func (b *jsonBuilder) precomputeAttrs(buf []byte, _ string, attrs []slog.Attr) []byte {
	for _, attr := range attrs {
		buf = b.appendAttr(buf, attr)
	}
	return buf
}

func (b *jsonBuilder) groupPrefix(oldPrefix []byte, newPrefix string) []byte {
	oldPrefix = slices.Grow(oldPrefix, len(newPrefix)+5)

	oldPrefix = b.addComma(oldPrefix)

	oldPrefix = append(oldPrefix, '"')
	oldPrefix = append(
		oldPrefix,
		newPrefix...) // dangerous because it does not track whether there are invalid JSON characters in the line
	oldPrefix = append(oldPrefix, `":{`...)

	return oldPrefix
}

func (b *jsonBuilder) addComma(buf []byte) []byte {
	if len(buf) > 0 {
		var last = buf[len(buf)-1]
		if last != '{' && last != ',' && last != '[' {
			buf = append(buf, ',')
		}
	}
	return buf
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
