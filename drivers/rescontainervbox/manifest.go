package rescontainervbox

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/converters"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/rescontainer"
)

var (
	//go:embed text
	fs    embed.FS
	drvID = driver.NewID(driver.GroupContainer, "vbox")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
		manifest.ContextPeers,
		manifest.ContextDNS,
		manifest.ContextTopology,
		rescontainer.KWRCmd,
		rescontainer.KWName,
		rescontainer.KWHostname,
		rescontainer.KWStartTimeout,
		rescontainer.KWStopTimeout,
		rescontainer.KWPromoteRW,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
		keywords.Keyword{
			Option:     "headless",
			Attr:       "Headless",
			Converter:  converters.Bool,
			Default:    "false",
			Text:       keywords.NewText(fs, "text/kw/headless"),
			Deprecated: "3.0",
		},
	)
	return m
}
