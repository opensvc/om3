package resfssgcp_nfs_cg

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

	drvID = driver.NewID(driver.GroupFS, "sgcp_nfs_cg")

	kws = []*keywords.Keyword{
		{
			Attr:     "UUID",
			Option:   "uuid",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/uuid"),
		},
		{
			Attr:     "AZ",
			Option:   "az",
			Default:  "{node.labels.az}",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/az"),
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
		{
			Attr:      "Timeout",
			Option:    "timeout",
			Converter: "duration",
			Default:   "300s", // TODO: move to config
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
		},
		{
			Attr:      "Failover",
			Option:    "failover",
			Converter: "boolean",
			Default:   "true",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/failover"),
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
