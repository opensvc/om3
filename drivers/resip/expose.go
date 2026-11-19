package resip

// exposeNone is the expose value saying the address exposes nothing, its
// published ports included.
const exposeNone = "none"

// ExposeInfo returns the expose value the status info of a resource with
// no publish keyword reports: none for "none".
func ExposeInfo(expose []string) []string {
	if len(expose) == 1 && expose[0] == exposeNone {
		return []string{}
	}
	return expose
}
