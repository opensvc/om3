package resipsgcp_dnsalias

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

	drvID = driver.NewID(driver.GroupIP, "sgcp_dnsalias")

	// GroupDNS holds the alias the resource points to the node. It has the
	// id, name and order of the dns group of the ip drivers, its text
	// saying the alias of the sgcp DNS rather than the records of the
	// cluster DNS.
	//
	// The uuid and the name are no strategies: the driver reads both
	// when both are set, the uuid to find the alias and the name to check
	// it, and refuses only a resource setting neither.
	GroupDNS = keywords.Group{
		ID:    "dns",
		Name:  "DNS",
		Text:  keywords.NewText(fs, "text/group/dns"),
		Order: 40,
	}

	// GroupAPI holds how the driver reaches the sgcp api.
	GroupAPI = keywords.Group{
		ID:    "api",
		Name:  "API",
		Text:  keywords.NewText(fs, "text/group/api"),
		Order: 50,
	}

	kws = []*keywords.Keyword{
		{
			Attr:         "UUID",
			Option:       "uuid",
			Group:        GroupDNS.ID,
			Required:     false,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/uuid"),
		},
		{
			Attr:         "Name",
			Option:       "name",
			Group:        GroupDNS.ID,
			Required:     false,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:        "Target",
			Option:      "target",
			Group:       GroupDNS.ID,
			Required:    false,
			Scopable:    true,
			DefaultText: keywords.NewText(fs, "text/kw/target.default"),
			Text:        keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:         "ZoneID",
			Option:       "zone_id",
			Group:        GroupDNS.ID,
			Required:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/zone_id"),
		},
		{
			Attr:     "Secret",
			Option:   "secret",
			Group:    GroupAPI.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/secret"),
		},
		{
			Attr:     "Endpoint",
			Option:   "endpoint",
			Group:    GroupAPI.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/endpoint"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Points an sgcp DNS alias to the node.")
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(kws...)
	m.AddGroups(GroupDNS, GroupAPI)
	return m
}
