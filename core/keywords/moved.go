package keywords

import (
	"github.com/opensvc/om3/v3/core/driver"
)

// moved is what a driver no longer reads, and what to write instead, by
// driver and by keyword.
//
// A keyword a driver dropped is not declared to carry the message. Declaring
// it would put it back in the keyword documentation, offered to whoever reads
// it, where the point is that om does not read it any more. So the table is
// consulted where a keyword is found not to exist, and only there.
//
// What it holds is one sentence, addressed to whoever wrote the
// configuration: what om no longer does, and what to write for it to do it
// again.
var moved = map[string]map[string]string{
	"fs": {
		"vg":             fsVolumeText,
		"size":           fsVolumeText,
		"create_options": fsVolumeText,
	},
}

const fsVolumeText = "a filesystem no longer makes the volume it mounts: describe the volume as a disk.lv resource, and point fs.dev at it with {disk#<n>.exposed_devs[0]}. \"om <path> config migrate\" writes that for you."

// Moved is what to write instead of a keyword a driver no longer reads, and
// says whether the keyword is one of those: a keyword nothing knows about is
// a keyword nothing can say where it went.
//
// The group answers for its drivers. A keyword moves out of a driver group as
// a whole here, which is what happened to the filesystem that made its own
// logical volume: every fs type had it.
func Moved(did driver.ID, option string) (string, bool) {
	byOption, ok := moved[did.Group.String()]
	if !ok {
		return "", false
	}
	text, ok := byOption[option]
	return text, ok
}
