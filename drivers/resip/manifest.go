package resip

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	KeywordWaitDNS = keywords.Keyword{
		Attr:      "WaitDNS",
		Converter: converters.Duration,
		Default:   "0",
		Example:   "10s",
		Option:    "wait_dns",
		Group:     GroupDNS.ID,
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/wait_dns"),
	}

	// KeywordPublish is the ports of the node forwarded to the address of
	// the ip drivers plumbing one in a network namespace, ip.netns and
	// ip.cni.
	KeywordPublish = keywords.Keyword{
		Attr:      "Publish",
		Converter: converters.List,
		Example:   "25489:80/tcp 192.168.1.10:8443:443/tcp",
		Option:    "publish",
		Group:     GroupPublish.ID,
		Scopable:  true,
		// The forwardings of a resource up are the ones of the value it
		// started with: a value changed or removed while it runs would
		// leave them in place, and its status would check the new one.
		StopToChange: true,
		Validate:     network.ValidatePublish,
		Text:         keywords.NewText(fs, "text/kw/publish"),
	}

	// GroupPublish is the group of the keywords saying the ports of the
	// node forwarded to the address.
	GroupPublish = keywords.Group{
		ID:    "publish",
		Name:  "Published ports",
		Text:  keywords.NewText(fs, "text/group/publish"),
		Order: 50,
	}

	// GroupAddressing is the group of the keywords saying the address of
	// the ip drivers plumbing one on an interface, ip.host and ip.netns:
	// written in the configuration, or drawn from an om network. The two
	// ways are its strategies, as a configuration naming an address and a
	// network says two addresses for one resource.
	//
	// The netmask is the static way's, but a network does not exclude
	// it: the netmask a configuration sets overrides the one of the
	// network.
	GroupAddressing = keywords.Group{
		ID:    "addressing",
		Name:  "Addressing",
		Text:  keywords.NewText(fs, "text/group/addressing"),
		Order: 10,
		// An ip naming neither an address nor a network to draw one from
		// has nothing a start can build.
		Required: true,
		Strategies: []keywords.Strategy{
			{
				ID:       "static",
				Name:     "Static address",
				Text:     keywords.NewText(fs, "text/strategy/addressing/static"),
				Uses:     []string{"name"},
				Excludes: []string{"network", "addr"},
				// The device of the node the address is plumbed
				// on, which a network says otherwise.
				Requires: []string{"dev"},
			},
			{
				ID:       "ipam",
				Name:     "Address drawn from a network",
				Text:     keywords.NewText(fs, "text/strategy/addressing/ipam"),
				Uses:     []string{"network", "addr"},
				Excludes: []string{"name"},
			},
		},
	}

	// GroupDNS is the group of the keywords saying the records the cluster
	// DNS serves for the address of the ip drivers handing one to the
	// daemon, ip.host, ip.netns and ip.cni: the SRV records of the
	// services it exposes, the suffix of its name, and whether a start
	// waits for them.
	GroupDNS = keywords.Group{
		ID:    "dns",
		Name:  "DNS",
		Text:  keywords.NewText(fs, "text/group/dns"),
		Order: 40,
	}
)
