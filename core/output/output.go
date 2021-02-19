package output

import (
	"bytes"
	"encoding/json"
)

// Type encodes as an integer one of the supported output formats
// (json, flat, human, table, csv)
type Type int

const (
	// Human encodes the prefered human friendly output format
	Human Type = iota
	// JSON encodes the json output format
	JSON
	// Flat encodes the flattened json output format (a.'b#b'.c = d, a[0] = b)
	Flat
	// Table encodes the simple tabular output format
	Table
	// CSV encodes the csv tabular output format
	CSV
)

var toString = map[Type]string{
	Human: "human",
	JSON:  "json",
	Flat:  "flat",
	Table: "table",
	CSV:   "csv",
}

var toID = map[string]Type{
	"human":     Human,
	"json":      JSON,
	"flat":      Flat,
	"flat_json": Flat, // compat
	"table":     Table,
	"csv":       CSV,
}

func (t Type) String() string {
	return toString[t]
}

// New returns the integer value of the output format
func New(s string) Type {
	return toID[s]
}

// MarshalJSON marshals the enum as a quoted json string
func (t Type) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Type) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = toID[j]
	return nil
}
