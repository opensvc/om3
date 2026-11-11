package datarecv

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords Keywords returns, declared once here and added
// by each driver installing files in its head, the filesystems and the
// volumes.
//
// The install lines and the configs, secrets and directories keywords they
// replace are no strategies: an install installs all of them, in order.
var (
	// GroupPermissions holds the owner and the permissions of the files
	// and directories installed, and of the root of the head.
	GroupPermissions = keywords.Group{
		ID:    "permissions",
		Name:  "Owner and permissions",
		Text:  keywords.NewText(fs, "text/group/permissions"),
		Order: 44,
	}

	// GroupInstall holds the files and directories installed in the head,
	// and the signals sent when they change. It has the id, name and order
	// of the install group of the tasks.
	GroupInstall = keywords.Group{
		ID:    "install",
		Name:  "Install",
		Text:  keywords.NewText(fs, "text/group/install"),
		Order: 46,
	}

	// Groups are the groups the keywords Keywords returns name.
	Groups = []keywords.Group{
		GroupPermissions,
		GroupInstall,
	}
)
