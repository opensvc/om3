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
			Option:   "file",
			Attr:     "File",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/file"),
			Example:  "/srv/{fqdn}-loop-{rindex}",
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Example:      "100m",
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
	)
	return m
}
