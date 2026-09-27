package rescontainerpodman

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
)

var (
	//go:embed text
	fs embed.FS
)

var (
	DrvID = driver.NewID(driver.GroupContainer, "podman")

	kws = []*keywords.Keyword{
		{
			Option:   "userns",
			Attr:     "UserNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/userns"),
		},
		{
			Option:   "rootless_user",
			Attr:     "RootlessUser",
			Scopable: true,
			Example:  "opensvc",
			Since:    "v3.0.0-rc42",
			Text:     keywords.NewText(fs, "text/kw/rootless_user"),
		},
		{
			Option:   "rootless_group",
			Attr:     "RootlessGroup",
			Scopable: true,
			Example:  "opensvc",
			Since:    "v3.0.0-rc42",
			Text:     keywords.NewText(fs, "text/kw/rootless_group"),
		},
	}
)

func init() {
	driver.Register(DrvID, New)
}

func (t *T) DriverID() driver.ID {
	return DrvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := t.BT.ManifestWithID(DrvID)
	m.AddKeywords(kws...)
	return m
}
