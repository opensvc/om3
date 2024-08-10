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
			Attr:     "ScriptPath",
			Option:   "script",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/script"),
		},
		{
			Attr:     "StartCmd",
			Option:   "start",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/start"),
		},
		{
			Attr:     "StopCmd",
			Option:   "stop",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/stop"),
		},
		{
			Attr:     "CheckCmd",
			Option:   "check",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/check"),
		},
		{
			Attr:     "InfoCmd",
			Default:  "false",
			Option:   "info",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/info"),
		},
		{
			Attr:      "StatusLogKw",
			Converter: converters.Bool,
			Default:   "false",
			Option:    "status_log",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/status_log"),
		},
		{
			Attr:      "CheckTimeout",
			Converter: converters.Duration,
			Example:   "180",
			Option:    "check_timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/check_timeout"),
		},
		{
			Attr:      "InfoTimeout",
			Converter: converters.Duration,
			Example:   "180",
			Option:    "info_timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/info_timeout"),
		},
		{
			Attr:     "Cwd",
			Option:   "cwd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/cwd"),
		},
		{
			Attr:     "User",
			Option:   "user",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/user"),
		},
		{
			Attr:     "Group",
			Option:   "group",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/group"),
		},
		{
			Attr:      "Limit.CPU",
			Converter: converters.Duration,
			Example:   "30s",
			Option:    "limit_cpu",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_cpu"),
		},
		{
			Attr:      "Limit.Core",
			Converter: converters.Size,
			Option:    "limit_core",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_core"),
		},
		{
			Attr:      "Limit.Data",
			Converter: converters.Size,
			Option:    "limit_data",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_data"),
		},
		{
			Attr:      "Limit.FSize",
			Converter: converters.Size,
			Option:    "limit_fsize",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_fsize"),
		},
		{
			Attr:      "Limit.MemLock",
			Converter: converters.Size,
			Option:    "limit_memlock",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_memlock"),
		},
		{
			Attr:      "Limit.NoFile",
			Converter: converters.Size,
			Option:    "limit_nofile",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_nofile"),
		},
		{
			Attr:      "Limit.NProc",
			Converter: converters.Size,
			Option:    "limit_nproc",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_nproc"),
		},
		{
			Attr:      "Limit.RSS",
			Converter: converters.Size,
			Option:    "limit_rss",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_rss"),
		},
		{
			Attr:      "Limit.Stack",
			Converter: converters.Size,
			Option:    "limit_stack",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_stack"),
		},
		{
			Attr:      "Limit.VMem",
			Converter: converters.Size,
			Option:    "limit_vmem",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_vmem"),
		},
		{
			Attr:      "Limit.AS",
			Converter: converters.Size,
			Option:    "limit_as",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/limit_as"),
		},
	}
)
