package converters

// Type defines a converter name
type Type string

const (
	// Shlex Convert string to a shell expression argv-style list
	Shlex Type = "shlex"
	// Integer Concert string to an integer
	Integer Type = "integer"
)
