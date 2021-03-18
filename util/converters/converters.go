package converters

// T defines a converter name
type T string

const (
	// Shlex Convert string to a shell expression argv-style list
	Shlex T = "shlex"
	// Integer Concert string to an integer
	Integer T = "integer"
)
