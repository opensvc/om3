package resfshost

import (
	"embed"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/filesystems"
)

var (
	//go:embed text
	fs embed.FS

	KeywordDevice = keywords.Keyword{
		Attr:     "Device",
		Default:  "none",
		Example:  "/dev/disk/by-id/nvme-eui.002538ba11b75ec8",
		Option:   "dev",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/dev"),
	}
	KeywordMKFSOptions = keywords.Keyword{
		Attr:         "MKFSOptions",
		Converter:    converters.Shlex,
		Default:      "",
		Option:       "mkfs_opt",
		Provisioning: true,
		Scopable:     true,
		Text:         keywords.NewText(fs, "text/kw/mkfs_opt"),
	}
	KeywordStatTimeout = keywords.Keyword{
		Attr:      "StatTimeout",
		Converter: converters.Duration,
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
		Converter: converters.Bool,
		Option:    "promote_rw",
		Text:      keywords.NewText(fs, "text/kw/promote_rw"),
	}
	KeywordZone = keywords.Keyword{
		Attr:     "Zone",
		Option:   "zone",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/zone"),
	}
	KeywordCheckReadDisabled = keywords.Keyword{
		Attr:      "CheckRead",
		Converter: converters.Bool,
		Option:    "check_read",
		Default:   "false",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/check_read"),
	}

	KeywordCheckReadEnabled = keywords.Keyword{
		Attr:      "CheckRead",
		Converter: converters.Bool,
		Option:    "check_read",
		Scopable:  true,
		Default:   "true",
		Text:      keywords.NewText(fs, "text/kw/check_read"),
	}

	// KeywordTmpfsSize is the size of a tmpfs.
	//
	// It is a keyword rather than the size option of mnt_opt so that a
	// resize records what it reached in it: a remount grows a tmpfs, and the
	// next mount reads the configuration again. A pool volume points it at
	// DEFAULT.size, the size the volume is claimed with.
	KeywordTmpfsSize = keywords.Keyword{
		Attr:      "Size",
		Converter: converters.Size,
		Example:   "1g",
		// A leaf keyword: a volume declares in DEFAULT the size it was
		// claimed with, and a resource inheriting it would be sized by it
		// without saying so.
		Inherit:      keywords.InheritLeaf,
		Option:       "size",
		Provisioning: true,
		Scopable:     true,
		Since:        "v3.0.0-rc42",
		Text:         keywords.NewText(fs, "text/kw/tmpfs.size"),
	}

	// KeywordTmpfsMode is the permissions of the root of a tmpfs.
	KeywordTmpfsMode = keywords.Keyword{
		Attr:      "Mode",
		Converter: converters.FileMode,
		Example:   "700",
		Inherit:   keywords.InheritLeaf,
		Option:    "mode",
		Scopable:  true,
		Since:     "v3.0.0-rc42",
		Text:      keywords.NewText(fs, "text/kw/tmpfs.mode"),
	}

	KeywordsBase = []*keywords.Keyword{
		&KeywordMountPoint,
		&KeywordDevice,
		&KeywordMountOptions,
		&KeywordStatTimeout,
		&KeywordPromoteRW,
		&KeywordMKFSOptions,
		&KeywordZone,
		&KeywordCheckReadDisabled,
	}
)

func init() {
	for _, t := range filesystems.Types() {
		driver.Register(driver.NewID(driver.GroupFS, t), NewF(t))
	}
}

func (t *T) DriverID() driver.ID {
	return driver.NewID(driver.GroupFS, t.Type)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(t.DriverID(), t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(KeywordsBase...)
	if t.isTmpfs() {
		m.AddKeywords(&KeywordTmpfsSize, &KeywordTmpfsMode)
	}
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.AddKeywords(datarecv.Keywords("DataRecv.")...)
	return m
}
