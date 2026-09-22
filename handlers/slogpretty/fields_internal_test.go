package slogpretty

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDecodeFieldsPreservesRawValues(t *testing.T) {
	input := []byte(`{"msg":"header","msg":"attribute","same":1,"same":2,"nested":{"key":1,"key":2},` +
		`"large":18446744073709551615,"escaped\"key":"100%\nготово"}`)
	fields, err := decodeFields(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := fields.takeString("msg"); got != "header" {
		t.Fatalf("got message %q", got)
	}
	indented, err := fields.indented()
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, indented); err != nil {
		t.Fatal(err)
	}
	want := bytes.Replace(input, []byte(`"msg":"header",`), nil, 1)
	if !bytes.Equal(compact.Bytes(), want) {
		t.Fatalf("fields changed:\ngot  %s\nwant %s", compact.Bytes(), want)
	}
}

func TestTakeStringPreservesNonStringMetadata(t *testing.T) {
	for _, value := range []string{"42", "false", "null", `{}`, `[]`} {
		t.Run(value, func(t *testing.T) {
			input := []byte(`{"msg":` + value + `,"msg":"user attribute"}`)
			fields, err := decodeFields(input)
			if err != nil {
				t.Fatal(err)
			}
			if got := fields.takeString("msg"); got != "" || len(fields) != 2 {
				t.Fatalf("non-string metadata or duplicate consumed: %q, %v", got, fields)
			}
		})
	}
}

func TestDecodeFieldsRejectsInvalidRecords(t *testing.T) {
	for _, input := range []string{
		"", "null", "[]", "42", `"text"`, "{", `{"key":}`, `{"key":1`,
		`{"key":1,}`, `{} {}`, `{} []`, `{} null`, `{} trailing`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := decodeFields([]byte(input)); err == nil {
				t.Fatalf("accepted invalid log record %q", input)
			}
		})
	}
}

func TestDecodeEmptyFields(t *testing.T) {
	fields, err := decodeFields([]byte("{}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fields.takeString("missing"); got != "" {
		t.Fatalf("unexpected field: %q", got)
	}
	if data, err := fields.indented(); err != nil || len(data) != 0 {
		t.Fatalf("unexpected empty fields: %s, %v", data, err)
	}
}
