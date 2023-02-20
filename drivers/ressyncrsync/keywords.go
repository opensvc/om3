package ressyncrsync

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	Keywords = []keywords.Keyword{
		{
			Option:        "schedule",
			DefaultOption: "run_schedule",
			Attr:          "Schedule",
			Scopable:      true,
			Text:          "Set the this task run schedule. See ``/usr/share/doc/opensvc/schedule`` for the schedule syntax reference.",
			Example:       "00:00-01:00 mon",
		},
		{
			Option:    "timeout",
			Attr:      "Timeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      "Wait for <duration> before declaring the sync action a failure. If no timeout is set, the agent waits indefinitely for the sync command to exit.",
			Example:   "5m",
		},
		{
			Option:   "src",
			Attr:     "Src",
			Scopable: true,
			//Required: true,
			Text:    "Source of the sync. Can be a whitespace-separated list of files or dirs passed as-is to rsync. Beware of the meaningful ending '/'. Refer to the rsync man page for details.",
			Example: "/srv/{fqdn}/",
		},
		{
			Option:   "dst",
			Attr:     "Dst",
			Scopable: true,
			Text:     "Destination of the sync. Beware of the meaningful ending '/'. Refer to the rsync man page for details.",
			Example:  "/srv/{fqdn}",
		},
		{
			Option:   "dstfs",
			Attr:     "DstFS",
			Scopable: true,
			Text:     "If set to a remote mount point, OpenSVC will verify that the specified mount point is really hosting a mounted FS. This can be used as a safety net to not overflow the parent FS (may be root).",
			Example:  "/srv/{fqdn}",
		},
		{
			Option:    "options",
			Attr:      "Options",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      "A whitespace-separated list of params passed unchanged to rsync. Typical usage is ACL preservation activation.",
			Example:   "--acls --xattrs --exclude foo/bar",
		},
		{
			Option:    "reset_options",
			Attr:      "ResetOptions",
			Converter: converters.Bool,
			Text:      "Use options as-is instead of appending options to default hardcoded options. Can be used to disable --xattr or --acls for example.",
		},
		{
			Option:     "target",
			Attr:       "Target",
			Converter:  converters.List,
			Candidates: []string{"nodes", "drpnodes"},
			Scopable:   true,
			//Required:   true,
			Text: "Describes which nodes should receive this data sync from the PRD node where the service is up and running. SAN storage shared 'nodes' must not be sync to 'nodes'. SRDF-like paired storage must not be sync to 'drpnodes'.",
		},
		{
			Option:    "snap",
			Attr:      "Snap",
			Converter: converters.Bool,
			Text:      "If set to ``true``, OpenSVC will try to snapshot the first snapshottable parent of the source of the sync and try to sync from the snap.",
		},
		{
			Option: "bwlimit",
			Attr:   "BandwidthLimit",
			Text:   "Bandwidth limit (the default unit is kb/s) applied to this rsync transfer. Leave empty to enforce no limit. Takes precedence over :kw:`bwlimit` set in [DEFAULT].",
		},
	}
)
