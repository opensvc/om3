package rescontainer

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	KWSCSIReserv = keywords.Keyword{
		Option:    "scsireserv",
		Attr:      "SCSIReserv",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to acquire a type-5 (write exclusive, registrant only) scsi3 persistent reservation on every path to every disks held by this resource. Existing reservations are preempted to not block service start-up. If the start-up was not legitimate the data are still protected from being written over from both nodes. If set to ``false`` or not set, :kw:`scsireserv` can be activated on a per-resource basis.",
	}
	KWPromoteRW = keywords.Keyword{
		Option:    "promote_rw",
		Attr:      "PromoteRW",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to promote the base devices to read-write on start.",
	}
	KWNoPreemptAbort = keywords.Keyword{
		Option:    "no_preempt_abort",
		Attr:      "NoPreemptAbort",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will preempt scsi reservation with a preempt command instead of a preempt and and abort. Some scsi target implementations do not support this last mode (esx). If set to ``false`` or not set, :kw:`no_preempt_abort` can be activated on a per-resource basis.",
	}
	KWOsvcRootPath = keywords.Keyword{
		Option:   "osvc_root_path",
		Attr:     "OsvcRootPath",
		Scopable: true,
		Text:     "If the OpenSVC agent is installed via package in the container, this parameter must not be set. Else the value can be set to the fullpath hosting the agent installed from sources.",
		Example:  "/opt/opensvc",
	}
	KWGuestOS = keywords.Keyword{
		Option:     "guest_os",
		Attr:       "GuestOS",
		Scopable:   true,
		Candidates: []string{"unix", "windows"},
		Text:       "The operating system in the virtual machine.",
		Example:    "unix",
		Default:    "unix",
	}
	KWRCmd = keywords.Keyword{
		Option:    "rcmd",
		Attr:      "RCmd",
		Scopable:  true,
		Converter: converters.Shlex,
		Text:      "The command to wrap another command to execute it in the container.",
		Example:   "lxc-attach -e -n osvtavnprov01 -- ",
	}
	KWName = keywords.Keyword{
		Option:   "name",
		Attr:     "Name",
		Required: true,
		Scopable: true,
		Text:     "The name to assign to the container",
	}
	KWHostname = keywords.Keyword{
		Option:   "hostname",
		Attr:     "Hostname",
		Scopable: true,
		Example:  "nginx1",
		Text:     "Set the container hostname. If not set, the container name is used.",
	}
	KWStartTimeout = keywords.Keyword{
		Option:    "start_timeout",
		Attr:      "StartTimeout",
		Scopable:  true,
		Converter: converters.Duration,
		Text:      "Wait for <duration> before declaring the container action a failure.",
		Example:   "1m5s",
		Default:   "4m",
	}
	KWStopTimeout = keywords.Keyword{
		Option:    "stop_timeout",
		Attr:      "StopTimeout",
		Scopable:  true,
		Converter: converters.Duration,
		Text:      "Wait for <duration> before declaring the container action a failure.",
		Example:   "2m30s",
		Default:   "2m",
	}
)
