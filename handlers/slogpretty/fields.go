package slogpretty

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

// Keep an ordered sequence rather than a map: slog permits duplicate keys and
// groups, including attributes with the same keys as the built-in metadata.
// Raw values also preserve nested duplicate keys and exact JSON numbers.
type recordField struct {
	key   string
	value json.RawMessage
}

type recordFields []recordField

func decodeFields(data []byte) (recordFields, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var fields recordFields
	if err := decodeObject(decoder, func(key string) error {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		fields = append(fields, recordField{key: key, value: value})
		return nil
	}); err != nil {
		return nil, err
	}
	if err := finishRecord(decoder); err != nil {
		return nil, err
	}
	return fields, nil
}

func decodeObject(decoder *json.Decoder, consume func(string) error) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return errors.New("log record must be a JSON object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("log field key must be a string")
		}
		if err := consume(key); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func finishRecord(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("multiple JSON values in log record")
	}
	return nil
}

// takeString moves a string from an already isolated built-in metadata scope to
// the header. Callers must establish provenance before calling it. Non-string
// replacements stay in the attribute object instead of being silently removed.
func (fields *recordFields) takeString(key string) string {
	for index, field := range *fields {
		if field.key != key {
			continue
		}
		value, ok := field.stringValue()
		if !ok {
			return ""
		}
		*fields = slices.Delete(*fields, index, index+1)
		return value
	}
	return ""
}

func (field recordField) stringValue() (string, bool) {
	if len(field.value) == 0 || field.value[0] != '"' {
		return "", false
	}
	var value string
	if err := json.Unmarshal(field.value, &value); err != nil {
		return "", false
	}
	return value, true
}

func (fields recordFields) indented() ([]byte, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	data := []byte{'{'}
	for index, field := range fields {
		key, err := json.Marshal(field.key)
		if err != nil {
			return nil, err
		}
		if index > 0 {
			data = append(data, ',')
		}
		data = append(data, key...)
		data = append(data, ':')
		data = append(data, field.value...)
	}
	data = append(data, '}')
	var output bytes.Buffer
	if err := json.Indent(&output, data, "", "  "); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
