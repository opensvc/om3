package resfssgcp_nfs

import (
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/drivers/resfshost"
)

// The groups of the keywords of an sgcp nfs filesystem beside the ones of
// the filesystems mounted, which it shares with them. The timeouts group has
// the id, name and order of the one of the containers and the tasks.
var (
	// GroupAccess holds the filesystem of the provider, and the access to
	// it the start grants the node.
	GroupAccess = keywords.Group{
		ID:    "access",
		Name:  "Access",
		Text:  keywords.NewText(fs, "text/group/access"),
		Order: 12,
	}

	// GroupAPI holds how the driver reaches the api of the provider.
	GroupAPI = keywords.Group{
		ID:    "api",
		Name:  "Provider api",
		Text:  keywords.NewText(fs, "text/group/api"),
		Order: 14,
	}

	// GroupTimeouts holds how long the start may take.
	GroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}

	// Groups are the groups the keywords of an sgcp nfs filesystem name.
	Groups = []keywords.Group{
		resfshost.GroupDevice,
		resfshost.GroupMount,
		resfshost.GroupStatus,
		GroupAccess,
		GroupAPI,
		GroupTimeouts,
	}
)
