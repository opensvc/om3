package resdiskloop

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resdisk"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupDisk, "loop")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Attr:     "File",
			Example:  "/srv/{fqdn}-loop-{rindex}",
			Option:   "file",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/file"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Example:      "100m",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
	)
	return m
}
