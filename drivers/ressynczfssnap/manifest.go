package ressynczfs

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
	drvID = driver.NewID(driver.GroupSync, "zfssnap")

	//go:embed text
	fs embed.FS

	Keywords = []*keywords.Keyword{
		{
			Attr:     "Name",
			Example:  "weekly",
			Option:   "name",
			Group:    ressync.GroupSnapshots.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:      "Dataset",
			Converter: converters.List,
			Example:   "svc1fs/data svc1fs/log",
			Option:    "dataset",
			Group:     ressync.GroupSnapshots.ID,
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/dataset"),
		},
		{
			Attr:      "Keep",
			Converter: converters.Int,
			Default:   "3",
			Example:   "3",
			Option:    "keep",
			Group:     ressync.GroupRetention.ID,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/keep"),
		},
		{
			Attr:      "Recursive",
			Converter: converters.Bool,
			Default:   "true",
			Option:    "recursive",
			Group:     ressync.GroupSnapshots.ID,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/recursive"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Takes and rotates ZFS dataset snapshots.")
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
	m.AddGroups(ressync.GroupSnapshots, ressync.GroupScheduling, ressync.GroupStatus, ressync.GroupRetention)
	return m
}
