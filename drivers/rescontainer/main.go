package rescontainer

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	KWPromoteRW = keywords.Keyword{
		Option:    "promote_rw",
		Attr:      "PromoteRW",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/promote_rw"),
	}
	KWOsvcRootPath = keywords.Keyword{
		Option:   "osvc_root_path",
		Attr:     "OsvcRootPath",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/osvc_root_path"),
		Example:  "/opt/opensvc",
	}
	KWGuestOS = keywords.Keyword{
		Option:     "guest_os",
		Aliases:    []string{"guestos"},
		Attr:       "GuestOS",
		Scopable:   true,
		Candidates: []string{"unix", "windows"},
		Text:       keywords.NewText(fs, "text/kw/guest_os"),
		Example:    "unix",
		Default:    "unix",
	}
	KWRCmd = keywords.Keyword{
		Option:    "rcmd",
		Attr:      "RCmd",
		Scopable:  true,
		Converter: converters.Shlex,
		Text:      keywords.NewText(fs, "text/kw/rcmd"),
		Example:   "lxc-attach -e -n osvtavnprov01 -- ",
	}
	KWName = keywords.Keyword{
		Option:   "name",
		Attr:     "Name",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/name"),
	}
	KWHostname = keywords.Keyword{
		Option:   "hostname",
		Attr:     "Hostname",
		Scopable: true,
		Example:  "nginx1",
		Text:     keywords.NewText(fs, "text/kw/hostname"),
	}
	KWStartTimeout = keywords.Keyword{
		Option:    "start_timeout",
		Attr:      "StartTimeout",
		Scopable:  true,
		Converter: converters.Duration,
		Text:      keywords.NewText(fs, "text/kw/start_timeout"),
		Example:   "1m5s",
		Default:   "4m",
	}
	KWStopTimeout = keywords.Keyword{
		Option:    "stop_timeout",
		Attr:      "StopTimeout",
		Scopable:  true,
		Converter: converters.Duration,
		Text:      keywords.NewText(fs, "text/kw/stop_timeout"),
		Example:   "2m30s",
		Default:   "2m",
	}
)
