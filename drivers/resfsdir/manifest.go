package resfsdir

import (
	"embed"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupFS, "directory")

	kws = []*keywords.Keyword{
		{
			Attr:     "Path",
			Option:   "path",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/path"),
		},
		{
			Attr:      "Size",
			Converter: converters.Size,
			// InheritLeaf, because a vol declares in its DEFAULT section the
			// size it was claimed from its pool. Inheriting that here would
			// give every directory of every volume a quota nobody asked for,
			// and fail on a filesystem that cannot hold one.
			Inherit:  keywords.InheritLeaf,
			Option:   "size",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/size"),
		},
		{
			Attr:      "ProjectID",
			Converter: converters.Int,
			Inherit:   keywords.InheritLeaf,
			Option:    "project_id",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/project_id"),
		},
		/*
			{
				Attr:     "Zone",
				Option:   "zone",
				Scopable: true,
				Text:     keywords.NewText(fs, "text/kw/zone"),
			},
		*/
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
	m.AddKeywords(kws...)
	m.AddKeywords(datarecv.Keywords("DataRecv.")...)
	return m
}
