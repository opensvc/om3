package resfsdir

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupFS, "directory")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "path",
			Attr:     "Path",
			Scopable: true,
			Required: true,
			Text:     "The fullpath of the directory to create.",
		},
		{
			Option:    "user",
			Attr:      "User",
			Scopable:  true,
			Converter: converters.User,
			Example:   "root",
			Text:      "The user that should be owner of the directory. Either in numeric or symbolic form.",
		},
		{
			Option:    "group",
			Attr:      "Group",
			Scopable:  true,
			Converter: converters.Group,
			Example:   "sys",
			Text:      "The group that should be owner of the directory. Either in numeric or symbolic form.",
		},
		{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Example:   "1777",
			Text:      "The permissions the directory should have. A string representing the octal permissions.",
		},
		keywords.Keyword{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     "The zone name the fs refers to. If set, the fs mount point is reparented into the zonepath rootfs.",
		},
	}...)
	return m
}
