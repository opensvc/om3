package restaskhost

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Option:        "schedule",
			DefaultOption: "run_schedule",
			Attr:          "Schedule",
			Scopable:      true,
			Text:          keywords.NewText(fs, "text/kw/schedule"),
			Example:       "00:00-01:00 mon",
		},
		{
			Option:    "timeout",
			Attr:      "Timeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
			Example:   "5m",
		},
		{
			Option:    "snooze",
			Attr:      "Snooze",
			Converter: converters.Duration,
			Scopable:  true,
			Example:   "10m",
			Text:      keywords.NewText(fs, "text/kw/snooze"),
		},
		{
			Option:    "log",
			Attr:      "LogOutputs",
			Default:   "true",
			Converter: converters.Bool,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/log"),
		},
		{
			Option:   "command",
			Attr:     "RunCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/command"),
		},
		{
			Option:   "on_error",
			Attr:     "OnErrorCmd",
			Scopable: true,
			Example:  "/srv/{name}/data/scripts/task_on_error.sh",
			Text:     keywords.NewText(fs, "text/kw/on_error"),
		},
		{
			Option:     "check",
			Attr:       "Check",
			Candidates: []string{"last_run", ""},
			Scopable:   true,
			Example:    "last_run",
			Text:       keywords.NewText(fs, "text/kw/check"),
		},
		{
			Option:    "confirmation",
			Attr:      "Confirmation",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/confirmation"),
		},
		{
			Option:    "run_timeout",
			Attr:      "RunTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Example:   "1m30s",
			Text:      keywords.NewText(fs, "text/kw/run_timeout"),
		},
	}
)
