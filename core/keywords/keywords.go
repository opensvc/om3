package keywords

import (
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/key"
)

// Keyword represents a configuration option in an object or node configuration file
type (
	Keyword struct {
		Section   string
		Option    string
		Attr      string
		Scopable  bool
		Required  bool
		Converter converters.T

		// Text is a text explaining the role of the keyword.
		Text string

		// DefaultText is a text explaining the default value.
		DefaultText string

		// Example demonstrates the keyword usage.
		Example string

		// Default is the value returned when the non-required keyword is not set.
		Default string

		// Candidates is the list of accepted values. An empty list
		Candidates []string
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
