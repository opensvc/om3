package resdiskzvol

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
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
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "name",
			Attr:     "Name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
			Example:  "tank/zvol1",
		},
		keywords.Keyword{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
			Example:      "-o dedup=on",
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/size"),
			Example:      "10m",
		},
		keywords.Keyword{
			Option:       "blocksize",
			Attr:         "BlockSize",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/blocksize"),
			Example:      "256k",
		},
	)
	return m
}
