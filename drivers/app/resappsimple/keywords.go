package resappsimple

import "opensvc.com/opensvc/core/keywords"

var (
	Keywords = []keywords.Keyword{
		{
			Option:     "kill",
			Attr:       "Kill",
			Scopable:   true,
			Required:   false,
			Text:       "Select a process kill strategy to use on resource stop. ``parent`` kill only the parent process forked by the agent. ``tree`` also kill its children.",
			Candidates: []string{"parent", "tree"},
			Default:    "parent",
		},
	}
)
