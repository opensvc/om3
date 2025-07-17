package restask

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Attr:       "Check",
			Candidates: []string{"last_run", "last_run_warn", ""},
			Example:    "last_run",
			Option:     "check",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/check"),
		},
		{
			Attr:      "Confirmation",
			Converter: "bool",
			Option:    "confirmation",
			Text:      keywords.NewText(fs, "text/kw/confirmation"),
		},
		{
			Attr:      "LogOutputs",
			Converter: "bool",
			Default:   "true",
			Option:    "log",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/log"),
		},
		{
			Attr:      "MaxParallel",
			Converter: "int",
			Default:   "1",
			Example:   "2",
			Option:    "max_parallel",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/max_parallel"),
		},
		{
			Attr:     "OnErrorCmd",
			Example:  "/srv/{name}/data/scripts/task_on_error.sh",
			Option:   "on_error",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/on_error"),
		},
		{
			Attr:     "RetCodes",
			Default:  "0:up 1:down",
			Example:  "0:up 1:down 3:warn 4: n/a 5:undef",
			Option:   "retcodes",
			Required: false,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/retcodes"),
		},
		{
			Attr:      "RunTimeout",
			Converter: "duration",
			Example:   "1m30s",
			Option:    "run_timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/run_timeout"),
		},
		{
			Attr:          "Schedule",
			DefaultOption: "run_schedule",
			Example:       "00:00-01:00 mon",
			Option:        "schedule",
			Scopable:      true,
			Text:          keywords.NewText(fs, "text/kw/schedule"),
		},
		{
			Attr:      "Snooze",
			Converter: "duration",
			Example:   "10m",
			Option:    "snooze",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/snooze"),
		},
		{
			Attr:      "Timeout",
			Converter: "duration",
			Example:   "5m",
			Option:    "timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
		},
	}
)
