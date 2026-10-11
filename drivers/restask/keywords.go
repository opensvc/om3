package restask

import (
	"embed"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []*keywords.Keyword{
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
			Converter: converters.Bool,
			Option:    "confirmation",
			Text:      keywords.NewText(fs, "text/kw/confirmation"),
		},
		{
			Attr:      "Install",
			Converter: converters.List,
			Example:   "fs#1",
			Option:    "install",
			Scopable:  true,
			Validate:  validInstall,
			Text:      keywords.NewText(fs, "text/kw/install"),
		},
		{
			Attr:      "LogOutputs",
			Converter: converters.Bool,
			Default:   "true",
			Option:    "log",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/log"),
		},
		{
			Attr:      "MaxParallel",
			Converter: converters.Int,
			Default:   "1",
			Example:   "2",
			Option:    "max_parallel",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/max_parallel"),
		},
		{
			Attr:      "On",
			Converter: converters.List,
			Example:   "NetworkPeersUpdated,name=backend1",
			Option:    "on",
			Scopable:  true,
			Validate:  validOn,
			Text:      keywords.NewText(fs, "text/kw/on"),
		},
		{
			Attr:     "Pace",
			Default:  schedule.DefaultPace,
			Example:  "@30s backoff @1m",
			Option:   "pace",
			Scopable: true,
			Validate: validPace,
			Text:     keywords.NewText(fs, "text/kw/pace"),
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
			Converter: converters.Duration,
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
			Converter: converters.Duration,
			Example:   "10m",
			Option:    "snooze",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/snooze"),
		},
		{
			Attr:      "Timeout",
			Converter: converters.Duration,
			Example:   "5m",
			Option:    "timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
		},
	}
)

// validOn refuses an on value naming an event no task runs on.
func validOn(v string) error {
	for _, s := range strings.Fields(v) {
		if _, err := schedule.ParseOn(s); err != nil {
			return err
		}
	}
	return nil
}

// validPace refuses a pace value not @<interval>[ backoff @<max>].
func validPace(v string) error {
	_, err := schedule.ParsePace(v)
	return err
}
