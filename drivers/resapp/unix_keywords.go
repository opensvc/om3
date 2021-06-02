// +build !windows

package resapp

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
)

var (
	UnixKeywords = []keywords.Keyword{
		{
			Option:   "script",
			Attr:     "ScriptPath",
			Scopable: true,
			Text: "Full path to the app launcher script. Or its basename if the file is hosted in " +
				" the ``<pathetc>/namespaces/<namespace>/<kind>/<name>.d/`` path." +
				" This script must accept as arg0 the activated actions word: ``start`` for start, ``stop`` for stop," +
				" ``status`` for check, ``info`` for info.",
		},
		{
			Option:   "start",
			Attr:     "StartCmd",
			Scopable: true,
			Text: "``true`` execute :cmd:`<script> start` on start action. ``false`` do nothing on start action." +
				" ``<shlex expression>`` execute the command on start.",
		},
		{
			Option:   "stop",
			Attr:     "StopCmd",
			Scopable: true,
			Text: "``true`` execute :cmd:`<script> stop` on stop action. ``false`` do nothing on stop action." +
				" ``<shlex expression>`` execute the command on stop action.",
		},
		{
			Option:   "check",
			Attr:     "CheckCmd",
			Scopable: true,
			Text: "``true`` execute :cmd:`<script> status` on status evaluation. ``false`` do nothing on status" +
				" evaluation. ``<shlex expression>`` execute the command on status evaluation.",
		},
		{
			Option:   "info", // TODO
			Attr:     "InfoCmd",
			Scopable: true,
			Text: "``true`` execute :cmd:`<script> info` on info action. ``false`` do nothing on info action." +
				" ``<shlex expression>`` execute the command on info action.",
			Default: "false",
		},
		{
			Option:    "status_log", // TODO
			Attr:      "StatusLogKw",
			Scopable:  true,
			Converter: converters.Bool,
			Text: "Redirect the checker script stdout to the resource status info-log, and stderr to warn-log." +
				" The default is ``false``, for it is common the checker scripts outputs are not tuned for opensvc.",
			Default: "false",
		},
		{
			Option:    "check_timeout", // TODO
			Attr:      "CheckTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text: "Wait for <duration> before declaring the app launcher check action a failure." +
				" Takes precedence over :kw:`timeout`. If neither :kw:`timeout` nor :kw:`check_timeout` is set," +
				" the agent waits indefinitely for the app launcher to return." +
				" A timeout can be coupled with :kw:`optional=true` to not abort a service instance check when an app" +
				" launcher did not return.",
			Example: "180",
		},
		{
			Option:    "info_timeout", // TODO
			Attr:      "InfoTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text: "Wait for <duration> before declaring the app launcher info action a failure." +
				" Takes precedence over :kw:`timeout`. If neither :kw:`timeout` nor :kw:`info_timeout` is set," +
				" the agent waits indefinitely for the app launcher to return. A timeout can be coupled with" +
				" :kw:`optional=true` to not abort a service instance info when an app launcher did not return.",
			Example: "180",
		},
		{
			Option:   "cwd",
			Attr:     "Cwd",
			Scopable: true,
			Text:     "Change the working directory to the specified location instead of the default ``<pathtmp>``.",
		},
		{
			Option:   "user",
			Attr:     "User",
			Scopable: true,
			Text:     "If the binary is owned by the root user, run it as the specified user instead of root.",
		},
		{
			Option:   "group",
			Attr:     "Group",
			Scopable: true,
			Text:     "If the binary is owned by the root user, run it as the specified group instead of root.",
		},
		{
			Option:    "limit_cpu",
			Attr:      "LimitCpu",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      "the limit on CPU time (duration).",
			Example:   "30s",
		},
		{
			Option:    "limit_core",
			Attr:      "LimitCore",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the largest core dump size that can be produced (unit byte).",
		},
		{
			Option:    "limit_data",
			Attr:      "LimitData",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the data segment size of a process (unit byte).",
		},
		{
			Option:    "limit_fsize",
			Attr:      "LimitFSize",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the largest file that can be created (unit byte).",
		},
		{
			Option:    "limit_memlock",
			Attr:      "LimitMemLock",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on how much memory a process can lock with mlock(2) (unit byte, no solaris support)",
		},
		{
			Option:    "limit_nofile",
			Attr:      "LimitNoFile",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the number files a process can have open at once.",
		},
		{
			Option:    "limit_nproc",
			Attr:      "LimitNProc",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the number of processes this user can have at one time, no solaris support",
		},
		{
			Option:    "limit_rss",
			Attr:      "LimitRss",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the total physical memory that can be in use by a process (unit byte).",
		},
		{
			Option:    "limit_stack",
			Attr:      "LimitStack",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the stack size of a process (unit bytes).",
		},
		{
			Option:    "limit_vmem",
			Attr:      "LimitVMem",
			Converter: converters.Size,
			Scopable:  true,
			Text:      "limit on the total virtual memory that can be in use by a process (unit bytes).",
		},
		{
			Option:    "limit_as",
			Attr:      "LimitAs",
			Converter: converters.Size,
			Scopable:  true,
			Text: "limit on the total virtual memory that can be in use by a process (unit bytes)" +
				" (same as limit_vmem). When both limit_vmem and limit_as is used, max value is chosen.",
		},
	}
)
