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
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, errors.New("log record must be a JSON object")
	}
	var fields recordFields
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("log field key must be a string")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields = append(fields, recordField{key: key, value: value})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("multiple JSON values in log record")
	}
	return fields, nil
}

// takeString moves only the first occurrence of a conventional metadata key to
// the header. JSONHandler emits built-ins before user attributes. Non-string
// replacements stay in the attribute object instead of being silently removed.
func (fields *recordFields) takeString(key string) string {
	for index, field := range *fields {
		if field.key != key {
			continue
		}
		if len(field.value) == 0 || field.value[0] != '"' {
			return ""
		}
		var value string
		if err := json.Unmarshal(field.value, &value); err != nil {
			return ""
		}
		*fields = slices.Delete(*fields, index, index+1)
		return value
	}
	return ""
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
