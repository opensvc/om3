package resdiskrados

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resdisk"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupDisk, "rados")
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
		manifest.ContextObjectFQDN,
		keywords.Keyword{
			Attr:     "Name",
			Example:  "pool1/cluster1/svc1",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Example:      "100m",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:         "Access",
			Candidates:   []string{"rwo", "roo", "rwx", "rox"},
			Default:      "rwo",
			Option:       "access",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/access"),
		},
		keywords.Keyword{
			Attr:     "Keyring",
			Option:   "keyring",
			Scopable: true,
			Example:  "from ./sec/ceph key eu1.keyring",
			Text:     keywords.NewText(fs, "text/kw/keyring"),
		},
	)
	return m
}
