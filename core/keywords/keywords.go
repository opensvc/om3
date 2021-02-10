package keywords

import (
        "opensvc.com/opensvc/core/converters"
)

type Keyword struct {
	Name		string
	Scopable	bool
	Required	bool
	Convert		converters.ConverterType
	Text		string
	Example		string
}

