package resappbase

import "opensvc.com/opensvc/core/keywords"

var (
	Keywords = []keywords.Keyword{
		{
			Option:   "retcodes",
			Attr:     "RetCodes",
			Scopable: true,
			Required: false,
			Text:     "The whitespace separated list of ``<retcode>:<status name>``. All undefined retcodes are mapped to the 'warn' status.",
			Default:  "0:up 1:down",
			Example:  "0:up 1:down 3:n/a",
		},
	}
)
