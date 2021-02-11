package converters

// Type defines a converter name
type Type string

const (
	// ConverterSHLEX Convert string to a shell expression argv-style list
	ConverterSHLEX Type = "shlex"
	// ConverterINTEGER Concert string to an integer
	ConverterINTEGER Type = "integer"
)
