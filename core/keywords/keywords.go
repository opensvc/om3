package keywords

import (
	"github.com/golang-collections/collections/set"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/key"
)

// Keyword represents a configuration option in an object or node configuration file
type (
	Keyword struct {
		Section    string
		Option     string
		Attr       string
		Scopable   bool
		Required   bool
		Converter  converters.T
		Text       string
		Example    string
		Default    string
		Candidates *set.Set
	}

	Store []Keyword
)

func (t Store) Lookup(k key.T) Keyword {
	for _, kw := range t {
		if k.Section == kw.Section && k.Option == kw.Option {
			return kw
		}
	}
	return Keyword{}
}

func (t Keyword) IsZero() bool {
	return t.Option == ""
}
