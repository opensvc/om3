package output

// T encodes as an integer one of the supported output formats
// (json, flat, human, table, csv)
type T int

const (
	// Human is the preferred human friendly custom output format
	Human T = iota
	// JSON is the json output format
	JSON
	// Flat is the flattened json output format (a."b#b".c = d, a[0] = b)
	Flat
	// JSONLine is unindented json output format
	JSONLine
	// Tab is the customizable tabular output format
	Tab
	// Table is the simple tabular output format
	Table
	// CSV is the csv tabular output format
	CSV
	// YAML is the standard human readable, commentable, complex data representation
	YAML
	// Template is a custom user provided renderer
	Template
)

var toString = map[T]string{
	Human:    "human",
	JSON:     "json",
	JSONLine: "jsonline",
	Flat:     "flat",
	Tab:      "tab",
	Table:    "table",
	CSV:      "csv",
	YAML:     "yaml",
	Template: "template",
}

var toID = map[string]T{
	"human":     Human,
	"json":      JSON,
	"jsonline":  JSONLine,
	"flat":      Flat,
	"flat_json": Flat, // compat
	"tab":       Tab,
	"table":     Table,
	"csv":       CSV,
	"yaml":      YAML,
	"template":  Template,
}

func (t T) String() string {
	return toString[t]
}

// New returns the integer value of the output format
func New(s string) T {
	return toID[s]
}

// MarshalText marshals the enum as a quoted json string
func (t T) MarshalText() ([]byte, error) {
	s := t.String()
	return []byte(s), nil
}

// UnmarshalText unmashals a quoted json string to the enum value
func (t *T) UnmarshalText(b []byte) error {
	s := string(b)
	*t = toID[s]
	return nil
}
