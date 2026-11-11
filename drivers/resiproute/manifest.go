package resiproute

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

	drvID = driver.NewID(driver.GroupIP, "route")

	// GroupRouting holds the route the resource adds, and the network
	// namespace it adds it in. It has the id, name and order of the
	// routing group of ip.netns.
	GroupRouting = keywords.Group{
		ID:    "routing",
		Name:  "Routing",
		Text:  keywords.NewText(fs, "text/group/routing"),
		Order: 30,
	}

	kws = []*keywords.Keyword{
		{
			Attr:         "NetNS",
			Example:      "container#0",
			Option:       "netns",
			Group:        GroupRouting.ID,
			References:   &keywords.References{Groups: []string{"container"}, Shape: keywords.ReferencesRID},
			Required:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/netns"),
		},
		{
			Attr:         "Gateway",
			Option:       "gateway",
			Group:        GroupRouting.ID,
			Example:      "1.2.3.4",
			Required:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/gateway"),
		},
		{
			Attr:         "To",
			Example:      "192.168.100.0/24",
			Option:       "to",
			Group:        GroupRouting.ID,
			Required:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/to"),
		},
		{
			Attr:         "Dev",
			DefaultText:  keywords.NewText(fs, "text/kw/dev.default"),
			Example:      "eth1",
			Option:       "dev",
			Group:        GroupRouting.ID,
			Required:     false,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/dev"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Adds a route, in the node or in a container network namespace.")
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.AddKeywords(kws...)
	m.AddGroups(GroupRouting)
	return m
}
