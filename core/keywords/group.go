package keywords

import (
	"embed"
	"slices"

	"github.com/opensvc/om3/v3/core/keyop"
)

type (
	// Group is a topic of the keywords of a section: the keywords a reader
	// looks at together, as the ones choosing the address of an ip. A
	// wizard shows the keywords of a section by group, the groups by
	// Order.
	//
	// Every keyword belongs to one group, named by its Group field. A
	// group is declared once, next to its keywords: the groups every
	// resource has next to the keywords every resource has, and a driver
	// adds its own to its manifest.
	Group struct {
		// ID is the name a keyword gives its group by, as "addressing".
		ID string

		// Name is the title of the group, as a wizard shows it.
		Name string

		// Text explains the topic of the group.
		Text string

		// Order is the rank of the group among the groups of a section,
		// the lowest first.
		Order int

		// Strategies are the ways to configure the group, when it has
		// alternatives that are not to be set together: an ip addressed
		// statically, or drawn from a network.
		Strategies []Strategy

		// Required says a section of a driver declaring the group follows
		// one of its strategies: one setting none of the keywords the
		// strategies use is refused by the validation, as an ip naming
		// neither an address nor a network to draw one from, which no
		// start can build.
		Required bool
	}

	// Strategy is one way to configure a group among its alternatives: the
	// keywords it sets, and the keywords of the other ways it leaves unset.
	// A configuration setting a keyword a strategy excludes next to one it
	// uses is warned of by the validation, as it says two ways at once.
	Strategy struct {
		// ID is the name of the strategy, as "static".
		ID string

		// Name is the title of the strategy, as a wizard shows it.
		Name string

		// Text explains when to choose the strategy, and what it does.
		Text string

		// Uses are the options of the keywords the strategy sets.
		Uses []string

		// Excludes are the options of the keywords the strategy leaves
		// unset, as they belong to another strategy.
		Excludes []string

		// Requires are the options of the keywords a section following
		// the strategy sets too, as the device of the node a static
		// address is plumbed on: the validation refuses the section
		// missing one. They may be of another group.
		Requires []string

		// When are the conditions on a selector keyword choosing the
		// strategy, as "topology=flex", written as the Depends of a
		// keyword. Empty when no keyword chooses it, the keywords set
		// telling which strategy a configuration follows.
		When []keyop.T
	}

	// DriverGroup is a group as the driver declaring it holds it: Driver is
	// the driver id, as "ip.host", empty for a group every resource has
	// and for the group of the keywords not grouped yet.
	DriverGroup struct {
		Driver string
		Group
	}
)

// GroupOtherID is the group of the keywords not grouped yet, which a
// keyword naming no group belongs to.
const GroupOtherID = "other"

//go:embed text
var fs embed.FS

// GroupOther is the group of a keyword naming none. Every keyword of the
// drivers, of the object kinds, of the node and of the cluster names one, as
// a test of the driverdb package verifies, so it holds none: it is what the
// api says of a keyword added without a group. It comes after the others.
var GroupOther = Group{
	ID:    GroupOtherID,
	Name:  "Other",
	Text:  NewText(fs, "text/group/other"),
	Order: 1000,
}

// GroupID returns the id of the group of the keyword, the one of the
// keywords not grouped yet when it names none.
func (t *Keyword) GroupID() string {
	if t.Group == "" {
		return GroupOtherID
	}
	return t.Group
}

// StrategyUsing returns the strategy of the group using the option, and whether
// one does.
func (t Group) StrategyUsing(option string) (Strategy, bool) {
	for _, s := range t.Strategies {
		if slices.Contains(s.Uses, option) {
			return s, true
		}
	}
	return Strategy{}, false
}

// SortDriverGroups sorts the groups as a wizard shows them: by order, then
// by id, a group every resource has before the one of a driver of the same
// id and order, and then by driver.
func SortDriverGroups(l []DriverGroup) {
	slices.SortFunc(l, func(a, b DriverGroup) int {
		switch {
		case a.Order != b.Order:
			return a.Order - b.Order
		case a.ID != b.ID:
			if a.ID < b.ID {
				return -1
			}
			return 1
		case a.Driver < b.Driver:
			return -1
		case a.Driver > b.Driver:
			return 1
		default:
			return 0
		}
	})
}
