package manifest

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords every resource has, or that several drivers
// share, declared next to these keywords.
//
// Their order starts at 100, after the groups a driver declares, which hold
// what the resource is about, as the address of an ip, and come first.
var (
	GroupAvailability = keywords.Group{
		ID:    "availability",
		Name:  "Availability",
		Text:  keywords.NewText(fs, "text/group/availability"),
		Order: 100,
	}

	GroupMonitoring = keywords.Group{
		ID:    "monitoring",
		Name:  "Monitoring",
		Text:  keywords.NewText(fs, "text/group/monitoring"),
		Order: 110,
	}

	GroupProvisioning = keywords.Group{
		ID:    "provisioning",
		Name:  "Provisioning",
		Text:  keywords.NewText(fs, "text/group/provisioning"),
		Order: 120,
	}

	GroupRequirements = keywords.Group{
		ID:    "requirements",
		Name:  "Requirements",
		Text:  keywords.NewText(fs, "text/group/requirements"),
		Order: 130,
	}

	GroupTriggers = keywords.Group{
		ID:    "triggers",
		Name:  "Triggers",
		Text:  keywords.NewText(fs, "text/group/triggers"),
		Order: 140,
	}

	GroupReservation = keywords.Group{
		ID:    "reservation",
		Name:  "Reservation",
		Text:  keywords.NewText(fs, "text/group/reservation"),
		Order: 150,
	}

	GroupSelection = keywords.Group{
		ID:    "selection",
		Name:  "Tags and subsets",
		Text:  keywords.NewText(fs, "text/group/selection"),
		Order: 160,
	}

	GroupEncap = keywords.Group{
		ID:    "encap",
		Name:  "Encapsulation",
		Text:  keywords.NewText(fs, "text/group/encap"),
		Order: 170,
	}

	// SharedGroups are the groups of the keywords every resource has, or
	// that several drivers share. A driver declares none of their ids.
	SharedGroups = []keywords.Group{
		GroupAvailability,
		GroupMonitoring,
		GroupProvisioning,
		GroupRequirements,
		GroupTriggers,
		GroupReservation,
		GroupSelection,
		GroupEncap,
	}
)

// SharedGroup returns the group of the id every resource has, the one of the
// keywords not grouped yet included, and whether there is one.
func SharedGroup(id string) (keywords.Group, bool) {
	if id == keywords.GroupOtherID {
		return keywords.GroupOther, true
	}
	for _, g := range SharedGroups {
		if g.ID == id {
			return g, true
		}
	}
	return keywords.Group{}, false
}
