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

	UnixKeywordScriptPath = keywords.Keyword{
		Attr:     "ScriptPath",
		Option:   "script",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/script"),
	}
	UnixKeywordStartCmd = keywords.Keyword{
		Attr:     "StartCmd",
		Option:   "start",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/start"),
	}
	UnixKeywordStopCmd = keywords.Keyword{
		Attr:     "StopCmd",
		Option:   "stop",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/stop"),
	}
	UnixKeywordCheckCmd = keywords.Keyword{
		Attr:     "CheckCmd",
		Option:   "check",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/check"),
	}
	UnixKeywordInfoCmd = keywords.Keyword{
		Attr:     "InfoCmd",
		Default:  "false",
		Option:   "info",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/info"),
	}
	UnixKeywordStatusLogKw = keywords.Keyword{
		Attr:      "StatusLogKw",
		Converter: converters.Bool,
		Default:   "false",
		Option:    "status_log",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/status_log"),
	}
	UnixKeywordCheckTimeout = keywords.Keyword{
		Attr:      "CheckTimeout",
		Converter: converters.Duration,
		Example:   "180",
		Option:    "check_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/check_timeout"),
	}
	UnixKeywordInfoTimeout = keywords.Keyword{
		Attr:      "InfoTimeout",
		Converter: converters.Duration,
		Example:   "180",
		Option:    "info_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/info_timeout"),
	}
	UnixKeywordCwd = keywords.Keyword{
		Attr:     "Cwd",
		Option:   "cwd",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/cwd"),
	}
	UnixKeywordUser = keywords.Keyword{
		Attr:     "User",
		Option:   "user",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/user"),
	}
	UnixKeywordGroup = keywords.Keyword{
		Attr:     "Group",
		Option:   "group",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/group"),
	}
	UnixKeywordLimitCPU = keywords.Keyword{
		Attr:      "Limit.CPU",
		Converter: converters.Duration,
		Example:   "30s",
		Option:    "limit_cpu",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_cpu"),
	}
	UnixKeywordLimitCore = keywords.Keyword{
		Attr:      "Limit.Core",
		Converter: converters.Size,
		Option:    "limit_core",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_core"),
	}
	UnixKeywordLimitData = keywords.Keyword{
		Attr:      "Limit.Data",
		Converter: converters.Size,
		Option:    "limit_data",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_data"),
	}
	UnixKeywordLimitFSize = keywords.Keyword{
		Attr:      "Limit.FSize",
		Converter: converters.Size,
		Option:    "limit_fsize",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_fsize"),
	}
	UnixKeywordLimitMemLock = keywords.Keyword{
		Attr:      "Limit.MemLock",
		Converter: converters.Size,
		Option:    "limit_memlock",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_memlock"),
	}
	UnixKeywordLimitNoFile = keywords.Keyword{
		Attr:      "Limit.NoFile",
		Converter: converters.Size,
		Option:    "limit_nofile",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_nofile"),
	}
	UnixKeywordLimitNProc = keywords.Keyword{
		Attr:      "Limit.NProc",
		Converter: converters.Size,
		Option:    "limit_nproc",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_nproc"),
	}
	UnixKeywordLimitRSS = keywords.Keyword{
		Attr:      "Limit.RSS",
		Converter: converters.Size,
		Option:    "limit_rss",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_rss"),
	}
	UnixKeywordLimitStack = keywords.Keyword{
		Attr:      "Limit.Stack",
		Converter: converters.Size,
		Option:    "limit_stack",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_stack"),
	}
	UnixKeywordLimitVmem = keywords.Keyword{
		Attr:      "Limit.VMem",
		Converter: converters.Size,
		Option:    "limit_vmem",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_vmem"),
	}
	UnixKeywordLimitAS = keywords.Keyword{
		Attr:      "Limit.AS",
		Converter: converters.Size,
		Option:    "limit_as",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/limit_as"),
	}
	UnixKeywords = []keywords.Keyword{
		UnixKeywordScriptPath,
		UnixKeywordStartCmd,
		UnixKeywordStopCmd,
		UnixKeywordCheckCmd,
		UnixKeywordInfoCmd,
		UnixKeywordStatusLogKw,
		UnixKeywordCheckTimeout,
		UnixKeywordInfoTimeout,
		UnixKeywordCwd,
		UnixKeywordUser,
		UnixKeywordGroup,
		UnixKeywordLimitCPU,
		UnixKeywordLimitCore,
		UnixKeywordLimitData,
		UnixKeywordLimitFSize,
		UnixKeywordLimitMemLock,
		UnixKeywordLimitNoFile,
		UnixKeywordLimitNProc,
		UnixKeywordLimitRSS,
		UnixKeywordLimitStack,
		UnixKeywordLimitVmem,
		UnixKeywordLimitAS,
	}
)
