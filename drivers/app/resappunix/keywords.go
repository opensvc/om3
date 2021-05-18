package resappunix

import "opensvc.com/opensvc/core/keywords"

var (
	Keywords = []keywords.Keyword{
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
