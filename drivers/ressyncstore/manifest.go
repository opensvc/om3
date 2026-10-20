package ressyncstore

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
	drvID = driver.NewID(driver.GroupSync, "store")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Attr:     "Store",
			Example:  "store1",
			Option:   "store",
			Required: true,
			Text:     keywords.NewText(fs, "text/kw/store"),
		},
		{
			Attr:      "RIDs",
			Converter: converters.List,
			Example:   "fs#1 volume#1",
			Option:    "rids",
			Text:      keywords.NewText(fs, "text/kw/rids"),
		},
		{
			Attr:      "Exclude",
			Converter: converters.List,
			Example:   "fs#1:/tmp *.log",
			Option:    "exclude",
			Text:      keywords.NewText(fs, "text/kw/exclude"),
		},
		{
			Attr:      "IncludeRefs",
			Converter: converters.Bool,
			Default:   "false",
			Option:    "include_refs",
			Text:      keywords.NewText(fs, "text/kw/include_refs"),
		},
		{
			Attr:    "Keep",
			Example: "24h 7d 4w 6m",
			Option:  "keep",
			Text:    keywords.NewText(fs, "text/kw/keep"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
	)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
