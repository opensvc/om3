package resfsdir

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of a directory beside the ones of the datarecv
// keywords, which hold its owner and permissions and the files installed in
// it.
var (
	// GroupDirectory holds the directory the resource creates. It has the
	// order of the device group of the filesystems mounted, which says
	// what they are too.
	GroupDirectory = keywords.Group{
		ID:    "directory",
		Name:  "Directory",
		Text:  keywords.NewText(fs, "text/group/directory"),
		Order: 10,
	}

	// GroupSize holds the project quota bounding what the directory
	// holds. It has the id, name and order of the size group of the
	// filesystems mounted.
	GroupSize = keywords.Group{
		ID:    "size",
		Name:  "Size",
		Text:  keywords.NewText(fs, "text/group/size"),
		Order: 28,
	}
)
