package rescontainervbox

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
)

var (
	//go:embed text
	fs    embed.FS
	drvID = driver.NewID(driver.GroupContainer, "vbox")

	kws = []*keywords.Keyword{
		&rescontainer.KWRCmd,
		&rescontainer.KWName,
		&rescontainer.KWHostname,
		&rescontainer.KWStartTimeout,
		&rescontainer.KWStopTimeout,
		&rescontainer.KWPromoteRW,
		&rescontainer.KWOsvcRootPath,
		&rescontainer.KWGuestOS,
		{
			Attr:       "Headless",
			Converter:  "bool",
			Default:    "false",
			Deprecated: "3.0",
			Option:     "headless",
			Text:       keywords.NewText(fs, "text/kw/headless"),
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
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
		manifest.ContextPeers,
		manifest.ContextDNS,
		manifest.ContextTopology,
	)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.AddKeywords(kws...)
	return m
}
