package config

import (
	"encoding/json"
	"fmt"

	"github.com/iancoleman/orderedmap"
)

type (
	Raw struct {
		Data *orderedmap.OrderedMap
	}
)

// MarshalJSON marshals the enum as a quoted json string
func (t Raw) MarshalJSON() ([]byte, error) {
	return t.Data.MarshalJSON()
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Raw) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, &t.Data)
	if err != nil {
		return err
	}
	return nil
}

// IsZero returns true if the Raw data has not been initialized
func (t Raw) IsZero() bool {
	return t.Data == nil
}

// Render return a colorized text version of the configuration file
func (t Raw) Render() string {
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
