package rescontainerkvm

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/rescontainer"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupContainer, "kvm")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
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
				Example:      "/srv/svc1/data/containers",
			},
			keywords.Keyword{
				Option:       "snapof",
				Attr:         "SnapOf",
				Scopable:     true,
				Provisioning: true,
				Text:         "Sets the root fs directory of the container",
				Example:      "/srv/svc1/data/containers",
			},
		*/
		keywords.Keyword{
			Option:       "virtinst",
			Attr:         "VirtInst",
			Provisioning: true,
			Converter:    converters.Shlex,
			Text:         "The arguments to pass through :cmd:`lxc-create` to the per-template script.",
			Example:      "--release focal",
		},
		rescontainer.KWRCmd,
		rescontainer.KWName,
		rescontainer.KWHostname,
		rescontainer.KWStartTimeout,
		rescontainer.KWStopTimeout,
		rescontainer.KWSCSIReserv,
		rescontainer.KWPromoteRW,
		rescontainer.KWNoPreemptAbort,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	return m
}
