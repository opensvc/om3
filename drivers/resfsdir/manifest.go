package resfsdir

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupFS, "directory")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		keywords.Keyword{
			Option:   "path",
			Attr:     "Path",
			Scopable: true,
			Required: true,
			Text:     keywords.NewText(fs, "text/kw/path"),
		},
		keywords.Keyword{
			Option:    "user",
			Attr:      "User",
			Scopable:  true,
			Converter: converters.User,
			Example:   "root",
			Text:      keywords.NewText(fs, "text/kw/user"),
		},
		keywords.Keyword{
			Option:    "group",
			Attr:      "Group",
			Scopable:  true,
			Converter: converters.Group,
			Example:   "sys",
			Text:      keywords.NewText(fs, "text/kw/group"),
		},
		keywords.Keyword{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Example:   "1777",
			Text:      keywords.NewText(fs, "text/kw/perm"),
		},
		keywords.Keyword{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
		},
	)
	return m
}
