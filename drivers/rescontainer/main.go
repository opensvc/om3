package rescontainer

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	KWPromoteRW = keywords.Keyword{
		Attr:      "PromoteRW",
		Converter: "bool",
		Option:    "promote_rw",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/promote_rw"),
	}
	KWOsvcRootPath = keywords.Keyword{
		Attr:     "OsvcRootPath",
		Example:  "/opt/opensvc",
		Option:   "osvc_root_path",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/osvc_root_path"),
	}
	KWGuestOS = keywords.Keyword{
		Aliases:    []string{"guestos"},
		Attr:       "GuestOS",
		Candidates: []string{"unix", "windows"},
		Default:    "unix",
		Example:    "unix",
		Option:     "guest_os",
		Scopable:   true,
		Text:       keywords.NewText(fs, "text/kw/guest_os"),
	}
	KWRCmd = keywords.Keyword{
		Attr:      "RCmd",
		Converter: "shlex",
		Example:   "lxc-attach -e -n osvtavnprov01 -- ",
		Option:    "rcmd",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/rcmd"),
		Default:   "/usr/bin/ssh -o StrictHostKeyChecking=accept-new -o ForwardX11=no",
	}
	KWName = keywords.Keyword{
		Attr:     "Name",
		Option:   "name",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/name"),
	}
	KWHostname = keywords.Keyword{
		Attr:     "Hostname",
		Example:  "nginx1",
		Option:   "hostname",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/hostname"),
	}
	KWStartTimeout = keywords.Keyword{
		Attr:      "StartTimeout",
		Converter: "duration",
		Default:   "4m",
		Example:   "1m5s",
		Option:    "start_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/start_timeout"),
	}
	KWStopTimeout = keywords.Keyword{
		Attr:      "StopTimeout",
		Converter: "duration",
		Default:   "2m",
		Example:   "2m30s",
		Option:    "stop_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/stop_timeout"),
	}
)
