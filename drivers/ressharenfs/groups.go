package ressharenfs

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// GroupExport holds the directory the resource exports, and the clients it
// exports it to with their options.
var GroupExport = keywords.Group{
	ID:    "export",
	Name:  "Export",
	Text:  keywords.NewText(fs, "text/group/export"),
	Order: 10,
}
