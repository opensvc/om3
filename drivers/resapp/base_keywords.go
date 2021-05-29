package resapp

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/util/converters"
)

var (
	BaseKeywords = []keywords.Keyword{
		{
			Option:    "timeout", // TODO
			Attr:      "Timeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text: "Wait for <duration> before declaring the app launcher action a failure." +
				" Can be overridden by :kw:`<action>_timeout`. If no timeout is set, the agent waits indefinitely for" +
				" the app launcher to return. A timeout can be coupled with :kw:`optional=true` to not abort a service" +
				" instance action when an app launcher did not return.",
			Example: "180",
		},
		{
			Option:    "start_timeout", // TODO
			Attr:      "StartTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text: "Wait for <duration> before declaring the app launcher start action a failure." +
				"  Takes precedence over :kw:`timeout`. If neither :kw:`timeout` nor :kw:`start_timeout` is set," +
				" the agent waits indefinitely for the app launcher to return." +
				" A timeout can be coupled with :kw:`optional=true to not abort a service instance start when an app" +
				" launcher did not return.",
			Example: "180",
		},
		{
			Option:    "stop_timeout", // TODO
			Attr:      "StopTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text: "Wait for <duration> before declaring the app launcher stop action a failure." +
				" Takes precedence over :kw:`timeout`. If neither :kw:`timeout` nor :kw:`stop_timeout` is set," +
				" the agent waits indefinitely for the app launcher to return." +
				" A timeout can be coupled with :kw:`optional=true` to not abort a service instance stop when an app" +
				" launcher did not return.",
			Example: "180",
		},
		{
			Option:    "secrets_environment",
			Attr:      "SecretsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text: "A whitespace separated list of ``<var>=<sec name>/<key path>`` or ``<sec name>/<key matcher>``." +
				" If secret object or secret key doesn't exist then start, stop, ... actions on resource will fail" +
				" with non 0 exit code." +
				" A shell expression splitter is applied, so double quotes can be around ``<secret name>/<key path>``" +
				" only or whole ``<var>=<secret name>/<key path>``.",
			Example: "``CRT=cert1/server.pem sec1/*`` to create following env vars CRT=< <ns>/sec/cert1 decoded" +
				" value of key server.pem> <key1>=< <ns>/sec/sec1 decoded value of <key1> ...",
		},
		{
			Option:    "configs_environment",
			Attr:      "ConfigsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text: "The whitespace separated list of ``<var>=<cfg name>/<key path>`` or ``<cfg name>/<key matcher>``." +
				" If config object or config key doesn't exist then start, stop, ... actions on resource will fail" +
				" with non 0 exit code." +
				" A shell expression splitter is applied, so double quotes can be around ``<config name>/<key path>``" +
				" only or whole ``<var>=<config name>/<key path>``.",
			Example: "``PORT=http/port webapp/app1* {name}/* {name}-debug/settings``",
		},
		{
			Option:    "environment",
			Attr:      "Env",
			Scopable:  true,
			Converter: converters.Shlex,
			Text: "The whitespace separated list of ``<var>=<config name>/<key path>``." +
				" A shell expression splitter is applied, so double quotes can be around ``<config name>/<key path>``" +
				" only or whole ``<var>=<config name>/<key path>``.",
			Example: "CRT=cert1/server.crt PEM=cert1/server.pem",
		},
		{
			Option:   "retcodes",
			Attr:     "RetCodes",
			Scopable: true,
			Required: false,
			Text: "The whitespace separated list of ``<retcode>:<status name>``." +
				" All undefined retcodes are mapped to the 'warn' status.",
			Default: "0:up 1:down",
			Example: "0:up 1:down 3:n/a",
		},
		{
			Option:    "umask",
			Attr:      "Umask",
			Scopable:  true,
			Converter: converters.Umask,
			Text:      "The umask to set for the application process.",
			Example:   "022",
		},
	}
)
