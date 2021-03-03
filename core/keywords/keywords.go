package keywords

import (
	"opensvc.com/opensvc/util/converters"
)

// Keyword represents a configuration option in an object or node configuration file
type Keyword struct {
	Name     string
	Scopable bool
	Required bool
	Convert  converters.Type
	Text     string
	Example  string
}
