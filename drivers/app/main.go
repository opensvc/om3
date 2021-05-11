package app

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
)

// T is the app base driver structure
type T struct {
	resource.T
	RetCodes string   `json:"retcodes"`
	Path     path.T   `json:"path"`
	Nodes    []string `json:"nodes"`
}

var (
	Keywords = []keywords.Keyword{
		{
			Option:   "retcodes",
			Attr:     "RetCodes",
			Scopable: true,
			Required: false,
			Text:     "The whitespace separated list of ``<retcode>=<status name>``. All undefined retcodes are mapped to the 'warn' status.",
			Default:  "0:up 1:down",
			Example:  "0:up 1:down 3:n/a",
		},
		{
			Option:   "start",
			Attr:     "StartCmd",
			Scopable: true,
			Text:     "``true`` execute :cmd:`<script> start` on start action. ``false`` do nothing on start action. ``<shlex expression>`` execute the command on start.",
		},
		{
			Option:   "stop",
			Attr:     "StopCmd",
			Scopable: true,
			Text:     "``true`` execute :cmd:`<script> stop` on stop action. ``false`` do nothing on stop action. ``<shlex expression>`` execute the command on stop action.",
		},
		{
			Option:   "check",
			Attr:     "CheckCmd",
			Scopable: true,
			Text:     "``true`` execute :cmd:`<script> status` on status evaluation. ``false`` do nothing on status evaluation. ``<shlex expression>`` execute the command on status evaluation.",
		},
	}
)
