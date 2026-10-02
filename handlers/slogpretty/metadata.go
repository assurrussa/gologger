package slogpretty

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
)

// Replacements travel through a private carrier so JSONHandler still handles
// groups and source specially, without invoking the user's ReplaceAttr twice.
type replacedAttr struct{ attr slog.Attr }

type groupAttr struct{ attr slog.Attr }

type frameEnd bool

type frameFlags uint8

const (
	timeHeader frameFlags = iota + 1
	levelHeader
	messageHeader
	groupValue  frameFlags = 1 << 2
	inlineGroup frameFlags = 1 << 3
	headerMask  frameFlags = groupValue - 1
	keepFrame   frameFlags = 1 << 4
	emptyGroup  frameFlags = 1 << 5
)

func frameMetadata(originalKey string, attr slog.Attr) slog.Attr {
	if attr.Key == "" && attr.Value.Kind() == slog.KindAny && attr.Value.Any() == nil {
		return slog.Attr{}
	}
	var flags frameFlags
	_, source := attr.Value.Any().(*slog.Source)
	if attr.Value.Kind() == slog.KindGroup || source {
		attr, flags = frameGroup(attr)
		flags |= groupValue | keepFrame
		if attr.Key == "" {
			flags |= inlineGroup
		}
	} else if attr.Key == originalKey {
		flags = headerForKey(originalKey)
	}
	// Frame names are transport flags. Original keys remain inside the frame,
	// so neither ordinary nor renamed user keys can impersonate metadata.
	if flags&keepFrame != 0 {
		return slog.Group(strconv.Itoa(int(flags)), slog.Any("", replacedAttr{attr}), slog.Any("", frameEnd(true)))
	}
	if flags != 0 {
		attr.Key = [...]string{"", "1:time", "2:level", "3:msg"}[flags]
	} else {
		attr.Key = "0:" + attr.Key
	}
	return attr
}

// frameGroup preserves the standard handler's lazy empty-group semantics while
// ensuring each nested value retains its own decoding boundary.
func frameGroup(attr slog.Attr) (slog.Attr, frameFlags) {
	if attr.Value.Kind() != slog.KindGroup {
		return attr, 0
	}
	children := attr.Value.Group()
	if len(children) == 0 {
		return attr, emptyGroup
	}
	framed := make([]slog.Attr, len(children))
	for index, child := range children {
		framed[index] = slog.Any("", groupAttr{child})
	}
	attr.Value = slog.GroupValue(framed...)
	return attr, 0
}

func decodeRecord(data []byte, framed bool) (recordFields, [3]string, error) {
	var headers [3]string
	var fields recordFields
	decoder := json.NewDecoder(bytes.NewReader(data))
	userField := func(key string) error {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		fields = append(fields, recordField{key: key, value: value})
		return nil
	}
	err := decodeObject(decoder, func(key string) error {
		// Read the user scope in the same decoder, avoiding a second buffer and
		// decoder while keeping its attributes out of metadata classification.
		if key == "attrs" {
			return decodeObject(decoder, userField)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		field := recordField{key: key, value: value}
		flags := headerForKey(key)
		if framed {
			var scalar bool
			var err error
			flags, field.key, scalar, err = frameInfo(field.key)
			if err != nil {
				return err
			}
			if !scalar {
				values, _, err := decodeFrame(recordField{key: key, value: value})
				if err != nil {
					return err
				}
				fields = append(fields, values...)
				return nil
			}
		}
		if header := flags & headerMask; header != 0 {
			if text, ok := field.stringValue(); ok {
				headers[header-1] = text
				return nil
			}
		}
		fields = append(fields, field)
		return nil
	})
	if err != nil {
		return nil, headers, err
	}
	if err := finishRecord(decoder); err != nil {
		return nil, headers, err
	}
	return fields, headers, nil
}

func headerForKey(key string) frameFlags {
	switch key {
	case slog.TimeKey:
		return timeHeader
	case slog.LevelKey:
		return levelHeader
	case slog.MessageKey:
		return messageHeader
	default:
		return 0
	}
}

func frameInfo(name string) (frameFlags, string, bool, error) {
	prefix, key, scalar := strings.Cut(name, ":")
	flags, err := strconv.ParseUint(prefix, 10, 8)
	return frameFlags(flags), key, scalar, err
}

func decodeFrame(field recordField) (recordFields, frameFlags, error) {
	flags, key, scalar, err := frameInfo(field.key)
	if err != nil {
		return nil, 0, err
	}
	if scalar {
		return recordFields{{key: key, value: field.value}}, flags, nil
	}
	fields, err := decodeFields(field.value)
	if err != nil {
		return nil, 0, err
	}
	if flags&keepFrame != 0 {
		fields = fields[:len(fields)-1]
	}
	if flags&groupValue == 0 || len(fields) == 0 {
		return fields, flags, nil
	}
	if flags&inlineGroup != 0 {
		values, present, err := expandFrames(fields)
		if present && len(values) == 0 {
			flags |= emptyGroup
		}
		return values, flags, err
	}
	nested, err := decodeFields(fields[0].value)
	if err != nil {
		return nil, 0, err
	}
	nested, present, err := expandFrames(nested)
	if err != nil {
		return nil, 0, err
	}
	if len(nested) == 0 {
		if !present {
			return nil, flags, nil
		}
		fields[0].value = []byte("{}")
		return fields, flags, nil
	}
	fields[0].value, err = nested.indented()
	return fields, flags, err
}

func expandFrames(fields recordFields) (recordFields, bool, error) {
	var expanded recordFields
	present := false
	for _, field := range fields {
		values, flags, err := decodeFrame(field)
		if err != nil {
			return nil, false, err
		}
		present = present || len(values) > 0 || flags&emptyGroup != 0
		expanded = append(expanded, values...)
	}
	return expanded, present, nil
}
