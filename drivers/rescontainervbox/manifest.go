package rescontainervbox

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/rescontainer"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupContainer, "vbox")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.Add(
		manifest.ContextPath,
		manifest.ContextObjectID,
		manifest.ContextPeers,
		manifest.ContextDNS,
		manifest.ContextTopology,
		/*
			keywords.Keyword{
				Option:       "snap",
				Attr:         "Snap",
				Scopable:     true,
				Provisioning: true,
				Text:         "If this keyword is set, the service configures a resource-private container data store. This setup is allows stateful service relocalization.",
				Text: keywords.NewText(fs, "text/kw/snap"),
				Example:      "/srv/svc1/data/containers",
			},
			keywords.Keyword{
				Option:       "snapof",
				Attr:         "SnapOf",
				Scopable:     true,
				Provisioning: true,
				Text:         "Sets the root fs directory of the container",
				Text: keywords.NewText(fs, "text/kw/snapof"),
				Example:      "/srv/svc1/data/containers",
			},
		*/
		rescontainer.KWRCmd,
		rescontainer.KWName,
		rescontainer.KWHostname,
		rescontainer.KWStartTimeout,
		rescontainer.KWStopTimeout,
		rescontainer.KWPromoteRW,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	return m
}
