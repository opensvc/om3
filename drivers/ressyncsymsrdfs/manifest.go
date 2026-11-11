package ressyncsymsrdfs

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
	drvID = driver.NewID(driver.GroupSync, "symsrdfs")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Attr:     "SymDG",
			Example:  "prod_db1",
			Option:   "symdg",
			Group:    ressync.GroupArray.ID,
			Required: true,
			Scopable: false,
			Text:     keywords.NewText(fs, "text/kw/symdg"),
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
			Attr:      "RDFG",
			Converter: converters.Int,
			Example:   "5",
			Option:    "rdfg",
			Group:     ressync.GroupArray.ID,
			Scopable:  false,
			Text:      keywords.NewText(fs, "text/kw/rdfg"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Fails over Dell PowerMax SRDF/S replication.")
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextDRPNodes,
	)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(kws...)
	m.AddGroups(ressync.GroupArray, ressync.GroupScheduling, ressync.GroupStatus)
	return m
}
