package resipsgcp_dnsalias

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "sgcp_dnsalias")

	kws = []*keywords.Keyword{
		{
			Attr:     "UUID",
			Option:   "uuid",
			Required: false,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/uuid"),
		},
		{
			Attr:     "Name",
			Option:   "name",
			Required: false,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:        "Target",
			Option:      "target",
			Required:    false,
			Scopable:    true,
			DefaultText: keywords.NewText(fs, "text/kw/target_default"),
			Text:        keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:     "ZoneID",
			Option:   "zone_id",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone_id"),
		},
		{
			Attr:     "Secret",
			Option:   "secret",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/secret"),
		},
		{
			Attr:     "Endpoint",
			Option:   "endpoint",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/endpoint"),
		},
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
	return m
}
