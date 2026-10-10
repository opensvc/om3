package ressyncbtrfssnap

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupSync, "btrfssnap")

	//go:embed text
	fs embed.FS

	Keywords = []*keywords.Keyword{
		{
			Attr:     "Name",
			Example:  "weekly",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:      "Subvol",
			Converter: converters.List,
			Example:   "svc1fs:data svc1fs:log",
			Option:    "subvol",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/subvol"),
		},
		{
			Attr:      "Keep",
			Converter: converters.Int,
			Default:   "3",
			Example:   "3",
			Option:    "keep",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/keep"),
		},
		{
			Attr:      "Recursive",
			Converter: converters.Bool,
			Default:   "false",
			Option:    "recursive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/recursive"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
