package resfshost

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/filesystems"
)

var (
	//go:embed text
	fs embed.FS

	KeywordDevice = keywords.Keyword{
		Attr:     "Device",
		Example:  "/dev/disk/by-id/nvme-eui.002538ba11b75ec8",
		Option:   "dev",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/dev"),
	}
	KeywordMKFSOptions = keywords.Keyword{
		Attr:         "MKFSOptions",
		Converter:    "shlex",
		Default:      "",
		Option:       "mkfs_opt",
		Provisioning: true,
		Scopable:     true,
		Text:         keywords.NewText(fs, "text/kw/mkfs_opt"),
	}
	KeywordStatTimeout = keywords.Keyword{
		Attr:      "StatTimeout",
		Converter: "duration",
		Default:   "5s",
		Option:    "stat_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/stat_timeout"),
	}
	KeywordMountPoint = keywords.Keyword{
		Attr:     "MountPoint",
		Example:  "/srv/{fqdn}",
		Option:   "mnt",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/mnt"),
	}
	KeywordMountOptions = keywords.Keyword{
		Attr:     "MountOptions",
		Option:   "mnt_opt",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/mnt_opt"),
	}
	KeywordPromoteRW = keywords.Keyword{
		Attr:      "PromoteRW",
		Converter: "bool",
		Option:    "promote_rw",
		Text:      keywords.NewText(fs, "text/kw/promote_rw"),
	}
	KeywordZone = keywords.Keyword{
		Attr:     "Zone",
		Option:   "zone",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/zone"),
	}
	KeywordUser = keywords.Keyword{
		Attr:      "User",
		Converter: "user",
		Example:   "root",
		Option:    "user",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/user"),
	}
	KeywordGroup = keywords.Keyword{
		Attr:      "Group",
		Converter: "group",
		Example:   "sys",
		Option:    "group",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/group"),
	}
	KeywordPerm = keywords.Keyword{
		Attr:      "Perm",
		Converter: "filemode",
		Example:   "1777",
		Option:    "perm",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/group"),
	}
	KeywordCheckRead = keywords.Keyword{
		Attr:      "CheckRead",
		Converter: "bool",
		Option:    "check_read",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/check_read"),
	}

	KeywordsVirtual = []keywords.Keyword{
		KeywordMountPoint,
		KeywordMountOptions,
		KeywordDevice,
		KeywordStatTimeout,
		KeywordZone,
		KeywordCheckRead,
	}

	KeywordsBase = []keywords.Keyword{
		KeywordMountPoint,
		KeywordDevice,
		KeywordMountOptions,
		KeywordStatTimeout,
		manifest.KWSCSIPersistentReservationKey,
		manifest.KWSCSIPersistentReservationEnabled,
		manifest.KWSCSIPersistentReservationNoPreemptAbort,
		KeywordPromoteRW,
		KeywordMKFSOptions,
		KeywordZone,
		KeywordUser,
		KeywordGroup,
		KeywordPerm,
		KeywordCheckRead,
	}

	KeywordsPooling = []keywords.Keyword{
		KeywordMountPoint,
		KeywordDevice,
		KeywordMountOptions,
		KeywordStatTimeout,
		KeywordMKFSOptions,
		KeywordZone,
		KeywordUser,
		KeywordGroup,
		KeywordPerm,
		KeywordCheckRead,
	}
)

func init() {
	for _, t := range filesystems.Types() {
		driver.Register(driver.NewID(driver.GroupFS, t), NewF(t))
	}
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(driver.NewID(driver.GroupFS, t.Type), t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(KeywordsBase...)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	return m
}
