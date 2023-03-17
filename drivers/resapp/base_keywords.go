package resapp

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	BaseKeywords = []keywords.Keyword{
		{
			Option:    "timeout",
			Attr:      "Timeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
			Example:   "180",
		},
		{
			Option:    "stop_timeout",
			Attr:      "StopTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/stop_timeout"),
			Example:   "180",
		},
		{
			Option:    "secrets_environment",
			Attr:      "SecretsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/secrets_environment"),
			Example:   "CRT=cert1/server.pem sec1/*",
		},
		{
			Option:    "configs_environment",
			Attr:      "ConfigsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/configs_environment"),
			Example:   "PORT=http/port webapp/app1* {name}/* {name}-debug/settings",
		},
		{
			Option:    "environment",
			Attr:      "Env",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/environment"),
			Example:   "CRT=cert1/server.crt PEM=cert1/server.pem",
		},
		{
			Option:   "retcodes",
			Attr:     "RetCodes",
			Scopable: true,
			Required: false,
			Text:     keywords.NewText(fs, "text/kw/retcodes"),
			Default:  "0:up 1:down",
			Example:  "0:up 1:down 3:warn 4: n/a 5:undef",
		},
		{
			Option:    "umask",
			Attr:      "Umask",
			Scopable:  true,
			Converter: converters.Umask,
			Text:      keywords.NewText(fs, "text/kw/umask"),
			Example:   "022",
		},
	}
)
