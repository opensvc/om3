package msgbus

import (
	"embed"
	"fmt"
	"reflect"
	"strings"
)

type (
	// Doc is the documentation of an event kind: Text says when it is
	// published and what it means, its first line a summary, and Fields
	// are the keys of its data.
	Doc struct {
		Kind   string  `json:"kind"`
		Text   string  `json:"text"`
		Fields []Field `json:"fields"`
	}

	// Field is a key of the data of an event, and the Go type of its value.
	Field struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
)

// texts holds the text of each event kind, text/kind/<Kind>.
//
//go:embed text/kind
var texts embed.FS

// Summary returns the first line of the text of the event kind, empty for a
// kind with no text.
func (t Doc) Summary() string {
	s, _, _ := strings.Cut(t.Text, "\n")
	return s
}

// Text returns the text of the event kind, false for a kind with none.
func Text(kind string) (string, bool) {
	b, err := texts.ReadFile("text/kind/" + kind)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

// Fields returns the keys of the data of the event kind, as an event of the
// stream carries them, and the Go type of their values.
func Fields(kind string) ([]Field, error) {
	v, err := KindToT(kind)
	if err != nil {
		return nil, err
	}
	typ := reflect.TypeOf(v)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	l := make([]Field, 0)
	if typ.Kind() != reflect.Struct {
		return l, nil
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Anonymous || !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		l = append(l, Field{Name: name, Type: f.Type.String()})
	}
	return l, nil
}

// GetDoc returns the documentation of the event kind.
func GetDoc(kind string) (Doc, error) {
	fields, err := Fields(kind)
	if err != nil {
		return Doc{}, fmt.Errorf("unknown event kind %s", kind)
	}
	text, _ := Text(kind)
	return Doc{Kind: kind, Text: text, Fields: fields}, nil
}
