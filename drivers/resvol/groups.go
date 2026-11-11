package resvol

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of a volume resource beside the ones of the
// datarecv keywords, which hold the owner and permissions of its root and the
// files installed in it.
//
// The pool group has no strategies: pool and type are no alternatives, a
// pool lookup keeping the pools matching both, see pool.Lookup.Do.
var (
	// GroupVolume holds the vol object the resource uses: its name, the
	// size it is claimed with, its nodes, and whether other objects use
	// it too.
	GroupVolume = keywords.Group{
		ID:    "volume",
		Name:  "Volume",
		Text:  keywords.NewText(fs, "text/group/volume"),
		Order: 10,
	}

	// GroupPool holds what the pool the vol is allocated from is chosen
	// by: its name, its type, and the capabilities it must have.
	GroupPool = keywords.Group{
		ID:    "pool",
		Name:  "Pool",
		Text:  keywords.NewText(fs, "text/group/pool"),
		Order: 20,
	}
)
