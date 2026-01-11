package logger

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"time"
)

func (h *ColorizedHandler) buildLog(buf []byte, record slog.Record) []byte {
	// Formatting: Time | Level | Message
	// Time
	buf = append(buf, h.colorOpts.TimeColor...) // color
	buf = record.Time.AppendFormat(buf, time.TimeOnly)
	buf = append(buf, reset...) // color

	buf = append(buf, " | "...)

	// Level
	levelColor := levelColor(record.Level) // color
	buf = append(buf, levelColor...)       // color
	buf = append(buf, levelBytes(record.Level)...)
	buf = append(buf, reset...) // color

	buf = append(buf, " | "...)

	// Message
	buf = append(buf, levelColor...) // color
	buf = append(buf, record.Message...)
	buf = append(buf, reset...) // color

	// Append precomputed attributes (from WithAttrs)
	if len(h.precomputed) > 0 {
		buf = append(buf, h.precomputed...)
	}
	// Process dynamic attributes (attached to this specific record)
	if record.NumAttrs() > 0 {
		// Stack-allocated buffer for group prefix to avoid allocs
		var groupBuf [128]byte
		pref := groupBuf[:0]

		// Add group from WithGroup()
		if len(h.groupPrefix) > 0 {
			pref = append(pref, h.groupPrefix...)
		}

		record.Attrs(func(attr slog.Attr) bool {
			buf = h.appendAttr(buf, pref, attr)
			return true
		})
	}

	buf = append(buf, '\n')
	return buf
}

func (h *ColorizedHandler) appendAttr(buf []byte, groupPrefix []byte, attr slog.Attr) []byte {
	// todo LogValuer
	//attr.Value = attr.Value.Resolve()

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
			buf = h.appendAttr(buf, groupPrefix, v)
		}
		return buf
	}

	buf = append(buf, ' ')
	buf = append(buf, h.colorOpts.KeyColor...) // color

	if len(groupPrefix) > 0 {
		buf = append(buf, groupPrefix...)
	}

	if attr.Key == "" {
		attr.Key = "!EMPTY_KEY"
	}
	buf = append(buf, attr.Key...)
	buf = append(buf, '=')
	buf = append(buf, reset...) // color

	buf = append(buf, h.colorOpts.ValueColor...) // color
	buf = h.writeValue(buf, attr.Value)
	buf = append(buf, reset...) // color

	return buf
}

func (h *ColorizedHandler) writeValue(buf []byte, value slog.Value) []byte {
	switch value.Kind() {
	case slog.KindString:
		str := value.String()
		if str == "" {
			str = "!EMPTY_VALUE"
		}
		buf = append(buf, str...)
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
		buf = append(buf, value.Duration().String()...) // todo проверить есть ли тут аллокация
	case slog.KindTime:
		buf = value.Time().AppendFormat(buf, time.RFC3339)
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
