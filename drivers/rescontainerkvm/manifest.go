package rescontainerkvm

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/rescontainer"
	"opensvc.com/opensvc/util/converters"
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
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
		{
			Key:  "object_id",
			Attr: "ObjectID",
			Ref:  "object.id",
		},
		{
			Key:  "peers",
			Attr: "Peers",
			Ref:  "object.nodes",
		},
		{
			Key:  "dns",
			Attr: "DNS",
			Ref:  "node.dns",
		},
		{
			Key:  "topology",
			Attr: "Topology",
			Ref:  "object.topology",
		},
	}...)
	m.AddKeyword([]keywords.Keyword{
		/*
			{
				Option:       "snap",
				Attr:         "Snap",
				Scopable:     true,
				Provisioning: true,
				Text:         "If this keyword is set, the service configures a resource-private container data store. This setup is allows stateful service relocalization.",
				Example:      "/srv/svc1/data/containers",
			},
			{
				Option:       "snapof",
				Attr:         "SnapOf",
				Scopable:     true,
				Provisioning: true,
				Text:         "Sets the root fs directory of the container",
				Example:      "/srv/svc1/data/containers",
			},
		*/
		{
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
	}...)
	return m
}
