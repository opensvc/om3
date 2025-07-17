package resfsdir

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		keywords.Keyword{
			Attr:     "Path",
			Option:   "path",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/path"),
		},
		keywords.Keyword{
			Attr:      "User",
			Converter: "user",
			Example:   "root",
			Option:    "user",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/user"),
		},
		keywords.Keyword{
			Attr:      "Group",
			Converter: "group",
			Example:   "sys",
			Option:    "group",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/group"),
		},
		keywords.Keyword{
			Attr:      "Perm",
			Converter: "filemode",
			Example:   "1777",
			Option:    "perm",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/perm"),
		},
		keywords.Keyword{
			Attr:     "Zone",
			Option:   "zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
		},
	)
	return m
}
