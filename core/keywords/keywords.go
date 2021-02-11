package keywords

import (
	"opensvc.com/opensvc/core/converters"
)

type Keyword struct {
	Name     string
	Scopable bool
	Required bool
	Convert  converters.Type
	Text     string
	Example  string
}
