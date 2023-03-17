package resfshost

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/filesystems"
)

var (
	//go:embed text
	fs embed.FS

	KeywordDevice = keywords.Keyword{
		Option:   "dev",
		Attr:     "Device",
		Scopable: true,
		Required: true,
		Text:     keywords.NewText(fs, "text/kw/dev"),
	}
	KeywordMKFSOptions = keywords.Keyword{
		Option:       "mkfs_opt",
		Attr:         "MKFSOptions",
		Converter:    converters.Shlex,
		Default:      "",
		Provisioning: true,
		Scopable:     true,
		Text:         keywords.NewText(fs, "text/kw/mkfs_opt"),
	}
	KeywordStatTimeout = keywords.Keyword{
		Option:    "stat_timeout",
		Attr:      "StatTimeout",
		Converter: converters.Duration,
		Default:   "5s",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/stat_timout"),
	}
	KeywordMountPoint = keywords.Keyword{
		Option:   "mnt",
		Attr:     "MountPoint",
		Scopable: true,
		Required: true,
		Text:     keywords.NewText(fs, "text/kw/mnt"),
	}
	KeywordMountOptions = keywords.Keyword{
		Option:   "mnt_opt",
		Attr:     "MountOptions",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/mnt_opt"),
	}
	KeywordPromoteRW = keywords.Keyword{
		Option:    "promote_rw",
		Attr:      "PromoteRW",
		Converter: converters.Bool,
		Text:      keywords.NewText(fs, "text/kw/promote_rw"),
	}
	KeywordZone = keywords.Keyword{
		Option:   "zone",
		Attr:     "Zone",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/zone"),
	}
	KeywordUser = keywords.Keyword{
		Option:    "user",
		Attr:      "User",
		Converter: converters.User,
		Scopable:  true,
		Example:   "root",
		Text:      keywords.NewText(fs, "text/kw/user"),
	}
	KeywordGroup = keywords.Keyword{
		Option:    "group",
		Attr:      "Group",
		Converter: converters.Group,
		Scopable:  true,
		Example:   "sys",
		Text:      keywords.NewText(fs, "text/kw/group"),
	}
	KeywordPerm = keywords.Keyword{
		Option:    "perm",
		Attr:      "Perm",
		Converter: converters.FileMode,
		Scopable:  true,
		Example:   "1777",
		Text:      keywords.NewText(fs, "text/kw/group"),
	}

	KeywordsVirtual = []keywords.Keyword{
		KeywordMountPoint,
		KeywordMountOptions,
		KeywordDevice,
		KeywordStatTimeout,
		KeywordZone,
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
	}
)

func init() {
	for _, t := range filesystems.Types() {
		driver.Register(driver.NewID(driver.GroupFS, t), NewF(t))
	}
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driver.NewID(driver.GroupFS, t.Type), t)
	m.Add(manifest.ContextPath)
	m.AddKeywords(KeywordsBase...)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	return m
}
