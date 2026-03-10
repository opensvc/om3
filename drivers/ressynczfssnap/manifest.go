package ressynczfs

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "zfssnap")

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
			Attr:      "Dataset",
			Converter: "list",
			Example:   "svc1fs/data svc1fs/log",
			Option:    "dataset",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/dataset"),
		},
		{
			Attr:      "Keep",
			Converter: "int",
			Default:   "3",
			Example:   "3",
			Option:    "keep",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/keep"),
		},
		{
			Attr:      "Recursive",
			Converter: "bool",
			Default:   "true",
			Option:    "recursive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/recursive"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
