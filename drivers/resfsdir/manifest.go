package resfsdir

import (
	"embed"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
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
