package resdiskzvol

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

	drvID = driver.NewID(driver.GroupDisk, "zvol")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Attr:     "Name",
			Example:  "tank/zvol1",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:         "CreateOptions",
			Converter:    "shlex",
			Example:      "-o dedup=on",
			Option:       "create_options",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Converter:    "size",
			Example:      "10m",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:         "BlockSize",
			Converter:    "size",
			Example:      "256k",
			Option:       "blocksize",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/blocksize"),
		},
	)
	return m
}
