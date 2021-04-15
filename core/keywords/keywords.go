package keywords

import (
	"github.com/golang-collections/collections/set"
	"opensvc.com/opensvc/util/key"
)

// Keyword represents a configuration option in an object or node configuration file
type (
	Converter interface {
		ToInt(string) (int, error)
		ToInt64(string) (int64, error)
		ToFloat(string) (float64, error)
		ToSlice(string) ([]string, error)
		ToSet(string) (*set.Set, error)
	}

	Keyword struct {
		Section   string
		Option    string
		Scopable  bool
		Required  bool
		Converter Converter
		Text      string
		Example   string
		Default   string
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
