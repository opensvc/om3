//go:build !windows

package resapp

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	UnixKeywords = []keywords.Keyword{
		{
			Option:   "script",
			Attr:     "ScriptPath",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/script"),
		},
		{
			Option:   "start",
			Attr:     "StartCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/start"),
		},
		{
			Option:   "stop",
			Attr:     "StopCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/stop"),
		},
		{
			Option:   "check",
			Attr:     "CheckCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/check"),
		},
		{
			Option:   "info",
			Attr:     "InfoCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/info"),
			Default:  "false",
		},
		{
			Option:    "status_log",
			Attr:      "StatusLogKw",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/status_log"),
			Default:   "false",
		},
		{
			Option:    "check_timeout",
			Attr:      "CheckTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/check_timeout"),
			Example:   "180",
		},
		{
			Option:    "info_timeout",
			Attr:      "InfoTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/info_timeout"),
			Example:   "180",
		},
		{
			Option:   "cwd",
			Attr:     "Cwd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/cwd"),
		},
		{
			Option:   "user",
			Attr:     "User",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/user"),
		},
		{
			Option:   "group",
			Attr:     "Group",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/group"),
		},
		{
			Option:    "limit_cpu",
			Attr:      "Limit.CPU",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_cpu"),
			Example:   "30s",
		},
		{
			Option:    "limit_core",
			Attr:      "Limit.Core",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_core"),
		},
		{
			Option:    "limit_data",
			Attr:      "Limit.Data",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_data"),
		},
		{
			Option:    "limit_fsize",
			Attr:      "Limit.FSize",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_fsize"),
		},
		{
			Option:    "limit_memlock",
			Attr:      "Limit.MemLock",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_memlock"),
		},
		{
			Option:    "limit_nofile",
			Attr:      "Limit.NoFile",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_nofile"),
		},
		{
			Option:    "limit_nproc",
			Attr:      "Limit.NProc",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_nproc"),
		},
		{
			Option:    "limit_rss",
			Attr:      "Limit.RSS",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_rss"),
		},
		{
			Option:    "limit_stack",
			Attr:      "Limit.Stack",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_stack"),
		},
		{
			Option:    "limit_vmem",
			Attr:      "Limit.VMem",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_vmem"),
		},
		{
			Option:    "limit_as",
			Attr:      "Limit.AS",
			Converter: converters.Size,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_as"),
		},
	}
)
