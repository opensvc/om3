package rescontainerpodman

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
)

var (
	//go:embed text
	fs embed.FS
)

var (
	drvID    = driver.NewID(driver.GroupContainer, "podman")
	altDrvID = driver.NewID(driver.GroupContainer, "oci")
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := t.BT.ManifestWithID(drvID)
	m.Add(
		keywords.Keyword{
			Option:   "userns",
			Attr:     "UserNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/userns"),
		},
	)
	return m
}
