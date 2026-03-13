package rescontainerkvm

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupContainer, "kvm")

	kws = []*keywords.Keyword{
		{
			Option:    "qga",
			Attr:      "QGA",
			Converter: "bool",
			Text:      keywords.NewText(fs, "text/kw/qga"),
			Scopable:  true,
		},
		{
			Option:    "qga_operational_delay",
			Attr:      "QGAOperationalDelay",
			Converter: "duration",
			Default:   "10s",
			Text:      "Wait after we successfully tested a pwd in the container, so the os is sufficiently started to accept a encap start.",
			Scopable:  true,
		},
		{
			Option:       "virtinst",
			Attr:         "VirtInst",
			Provisioning: true,
			Converter:    "shlex",
			Text:         keywords.NewText(fs, "text/kw/virtinst"),
			Example:      "--release focal",
		},
		&rescontainer.KWRCmd,
		&rescontainer.KWName,
		&rescontainer.KWHostname,
		&rescontainer.KWStartTimeout,
		&rescontainer.KWStopTimeout,
		&rescontainer.KWPromoteRW,
		&rescontainer.KWOsvcRootPath,
		&rescontainer.KWGuestOS,
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
		manifest.ContextEncapNodes,
		manifest.ContextPeers,
		manifest.ContextDNS,
		manifest.ContextTopology,
	)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.AddKeywords(kws...)
	return m
}
