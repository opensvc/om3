package object

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/key"
)

var keywordStore = keywords.Store{
	{
		Section:   "DEFAULT",
		Option:    "nodes",
		Scopable:  false,
		Required:  false,
		Converter: converters.List,
		Text:      "",
		Example:   "n1 n2",
	},
	{
		Section:   "DEFAULT",
		Option:    "drpnodes",
		Scopable:  false,
		Required:  false,
		Converter: converters.List,
		Text:      "",
		Example:   "n1 n2",
	},
	{
		Section:   "DEFAULT",
		Option:    "encapnodes",
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
