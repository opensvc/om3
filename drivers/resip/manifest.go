package resip

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
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
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/wait_dns"),
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
		Strategies: []keywords.Strategy{
			{
				ID:       "static",
				Name:     "Static address",
				Text:     keywords.NewText(fs, "text/strategy/addressing/static"),
				Uses:     []string{"name"},
				Excludes: []string{"network", "addr"},
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
)
