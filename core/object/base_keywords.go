package object

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/key"
)

var keywordStore = keywords.Store{
	{
		Option:    "nodes",
		Section:   "DEFAULT",
		Scopable:  false,
		Required:  false,
		Converter: converters.List,
		Text:      "",
		Example:   "n1 n2",
	},
	{
		Option:    "drpnodes",
		Section:   "DEFAULT",
		Scopable:  false,
		Required:  false,
		Converter: converters.List,
		Text:      "",
		Example:   "n1 n2",
	},
	{
		Option:    "encapnodes",
		Section:   "DEFAULT",
		Scopable:  false,
		Required:  false,
		Converter: converters.List,
		Text:      "",
		Example:   "n1 n2",
	},
}

func (t Base) KeywordLookup(k key.T) keywords.Keyword {
	return keywordStore.Lookup(k)
}
