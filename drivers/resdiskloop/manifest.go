package resdiskloop

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/resdisk"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "loop")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "file",
			Attr:     "File",
			Required: true,
			Scopable: true,
			Text:     "The loopback device backing file full path.",
			Example:  "/srv/{fqdn}-loop-{rindex}",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         "The size of the loop file to provision.",
			Example:      "100m",
		},
	}...)
	return m
}
