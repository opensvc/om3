package ressyncsymsnapvx

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
	drvID = driver.NewID(driver.GroupSync, "symsnapvx")

	//go:embed text
	fs embed.FS

	Keywords = []*keywords.Keyword{
		{
			Attr:     "Name",
			Example:  "prod_db1_weekly",
			Option:   "name",
			Group:    ressync.GroupSnapshots.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:     "SymID",
			Example:  "0000001234",
			Option:   "symid",
			Group:    ressync.GroupArray.ID,
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/symid"),
		},
		{
			Attr:      "Devices",
			Converter: converters.List,
			Example:   "012a 012b",
			Option:    "devs",
			Group:     ressync.GroupArray.ID,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/devs"),
		},
		{
			Attr:       "DevicesFrom",
			Converter:  converters.List,
			Example:    "disk#0 disk#1",
			Option:     "devs_from",
			Group:      ressync.GroupArray.ID,
			References: &keywords.References{Groups: []string{"disk", "fs", "volume", "container"}, Shape: keywords.ReferencesRIDs},
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/devs_from"),
		},
		{
			Attr:      "Secure",
			Converter: converters.Bool,
			Option:    "secure",
			Group:     ressync.GroupSnapshots.ID,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/secure"),
		},
		{
			Attr:     "Absolute",
			Example:  "12:15",
			Option:   "absolute",
			Group:    ressync.GroupSnapshots.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/absolute"),
		},
		{
			Attr:     "Delta",
			Example:  "00:15",
			Option:   "delta",
			Group:    ressync.GroupSnapshots.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/delta"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Takes Dell PowerMax SnapVX snapshots.")
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.Add(manifest.ContextObjectFQDN)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	m.AddGroups(ressync.GroupArray, ressync.GroupSnapshots, ressync.GroupScheduling, ressync.GroupStatus)
	return m
}
