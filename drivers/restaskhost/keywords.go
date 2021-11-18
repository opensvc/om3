package restaskhost

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
)

var (
	Keywords = []keywords.Keyword{
		{
			Option:    "timeout",
			Attr:      "Timeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      "Wait for <duration> before declaring the task run action a failure. If no timeout is set, the agent waits indefinitely for the task command to exit.",
			Example:   "5m",
		},
		{
			Option:    "snooze",
			Attr:      "Snooze",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      "Snooze the service before running the task, so if the command is known to cause a service status degradation the user can decide to snooze alarms for the duration set as value.",
			Example:   "10m",
		},
		{
			Option:    "log",
			Attr:      "LogOutputs",
			Converter: converters.Bool,
			Text:      "Log the task outputs in the service log.",
		},
		{
			Option:   "command",
			Attr:     "RunCmd",
			Scopable: true,
			Text:     "The shlex expression> to execute on run.",
		},
		{
			Option:   "on_error",
			Attr:     "OnErrorCmd",
			Scopable: true,
			Text:     "A command to execute on :c-action:`run` action if :kw:`command` returned an error.",
			Example:  "/srv/{name}/data/scripts/task_on_error.sh",
		},
		{
			Option:     "check",
			Attr:       "Check",
			Candidates: []string{"last_run", ""},
			Scopable:   true,
			Text:       "If set to 'last_run', the last run retcode is used to report a task resource status. If not set (default), the status of a task is always n/a.",
			Example:    "last_run",
		},
		{
			Option:    "confirmation",
			Attr:      "Confirmation",
			Converter: converters.Bool,
			Text:      "If set to True, ask for an interactive confirmation to run the task. This flag can be used for dangerous tasks like data-restore.",
		},
	}
)
