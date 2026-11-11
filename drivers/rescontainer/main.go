package rescontainer

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	KWPromoteRW = keywords.Keyword{
		Attr:       "PromoteRW",
		Converter:  converters.Bool,
		Deprecated: "v3.0.0-rc62",
		Group:      GroupStorage.ID,
		Option:     "promote_rw",
		Scopable:   true,
		Text:       keywords.NewText(fs, "text/kw/promote_rw"),
	}
	KWOsvcRootPath = keywords.Keyword{
		Attr:     "OsvcRootPath",
		Example:  "/opt/opensvc",
		Group:    GroupGuest.ID,
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
		Group:      GroupGuest.ID,
		Option:     "guest_os",
		Scopable:   true,
		Text:       keywords.NewText(fs, "text/kw/guest_os"),
	}
	KWRCmd = keywords.Keyword{
		Attr:      "RCmd",
		Converter: converters.Shlex,
		Example:   "lxc-attach -e -n osvtavnprov01 -- ",
		Group:     GroupGuest.ID,
		Option:    "rcmd",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/rcmd"),
		Default:   "/usr/bin/ssh -o StrictHostKeyChecking=accept-new -o ForwardX11=no",
	}
	KWName = keywords.Keyword{
		Attr:         "Name",
		Group:        GroupIdentity.ID,
		Option:       "name",
		Required:     true,
		Scopable:     true,
		StopToChange: true,
		Text:         keywords.NewText(fs, "text/kw/name"),
	}
	KWHostname = keywords.Keyword{
		Attr:     "Hostname",
		Example:  "nginx1",
		Group:    GroupIdentity.ID,
		Option:   "hostname",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/hostname"),
	}
	KWStartTimeout = keywords.Keyword{
		Attr:      "StartTimeout",
		Converter: converters.Duration,
		Default:   "4m",
		Example:   "1m5s",
		Group:     GroupTimeouts.ID,
		Option:    "start_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/start_timeout"),
	}
	KWStopTimeout = keywords.Keyword{
		Attr:      "StopTimeout",
		Converter: converters.Duration,
		Default:   "2m",
		Example:   "2m30s",
		Group:     GroupTimeouts.ID,
		Option:    "stop_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/stop_timeout"),
	}
)
