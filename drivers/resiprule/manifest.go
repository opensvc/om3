package resiprule

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "rule")

	// GroupRouting holds the policy routing rule the resource adds, and
	// the network namespace it adds it in. It has the id, name and order
	// of the routing group of ip.netns.
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
			Attr:         "Spec",
			Converter:    converters.Shlex,
			Example:      "from 192.168.100.0/24 table 100",
			Option:       "spec",
			Group:        GroupRouting.ID,
			Required:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/spec"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Adds a policy routing rule.")
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
