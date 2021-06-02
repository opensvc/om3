package rawconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/iancoleman/orderedmap"
)

type (
	T struct {
		Data *orderedmap.OrderedMap
	}
)

var (
	RegexpScope     = regexp.MustCompile(`(@[\w.-_]+)`)
	RegexpReference = regexp.MustCompile(`({[\w.-_:]+})`)
)

// MarshalJSON marshals the enum as a quoted json string
func (t T) MarshalJSON() ([]byte, error) {
	return t.Data.MarshalJSON()
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, &t.Data)
	if err != nil {
		return err
	}
	return nil
}

// IsZero returns true if the Raw data has not been initialized
func (t T) IsZero() bool {
	return t.Data == nil
}

// Render return a colorized text version of the configuration file
func (t T) Render() string {
	s := ""
	if t.Data == nil {
		return s
	}
	for _, section := range t.Data.Keys() {
		if section == "metadata" {
			continue
		}
		s += Node.Colorize.Primary(fmt.Sprintf("[%s]\n", section))
		data, _ := t.Data.Get(section)
		omap := data.(orderedmap.OrderedMap)
		for _, k := range omap.Keys() {
			v, _ := omap.Get(k)
			if k == "comment" {
				s += renderComment(k, v)
				continue
			}
			s += renderKey(k, v)
		}
		s += "\n"
	}
	return s
}

func renderComment(k string, v interface{}) string {
	vs, ok := v.(string)
	if !ok {
		return ""
	}
	return "# " + strings.ReplaceAll(vs, "\n", "\n# ") + "\n"
}

func renderKey(k string, v interface{}) string {
	k = RegexpScope.ReplaceAllString(k, Node.Colorize.Error("$1"))
	vs, ok := v.(string)
	if ok {
		vs = RegexpReference.ReplaceAllString(vs, Node.Colorize.Optimal("$1"))
		vs = strings.ReplaceAll(vs, "\n", "\n\t")
	} else {
		vs = ""
	}
	return fmt.Sprintf("%s = %s\n", Node.Colorize.Secondary(k), vs)
}
