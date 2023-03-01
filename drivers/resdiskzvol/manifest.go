package resdiskzvol

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "zvol")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "name",
			Attr:     "Name",
			Required: true,
			Scopable: true,
			Text:     "The full name of the zfs volume in the ``<pool>/<name>`` form.",
			Example:  "tank/zvol1",
		},
		keywords.Keyword{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         "The :cmd:`zfs create -V <name>` extra options.",
			Example:      "-o dedup=on",
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The size of the zfs volume to create.",
			Example:      "10m",
		},
		keywords.Keyword{
			Option:       "blocksize",
			Attr:         "BlockSize",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The blocksize of the zfs volume to create.",
			Example:      "256k",
		},
	)
	return m
}
