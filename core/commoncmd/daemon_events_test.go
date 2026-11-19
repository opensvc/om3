package commoncmd

import (
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// unlistedEventKinds are the event kinds registered but never published,
// which the help of the event filter does not list.
var unlistedEventKinds = []string{"Exit"}

// The help of the event filter lists every event kind the daemon publishes,
// and none it does not know.
func TestTheEventFilterHelpListsEveryKind(t *testing.T) {
	listed := regexp.MustCompile(`(?m)^ {4,}([A-Z][A-Za-z, ]+)$`).FindAllStringSubmatch(UsageFlagEventFilter(), -1)
	inHelp := make(map[string]bool)
	for _, m := range listed {
		for _, w := range regexp.MustCompile(`[A-Z][A-Za-z]+`).FindAllString(m[1], -1) {
			inHelp[w] = true
		}
	}
	kinds := msgbus.Kinds()
	for _, k := range kinds {
		if slices.Contains(unlistedEventKinds, k) {
			continue
		}
		assert.True(t, inHelp[k], "the event kind %s is not in the help of the event filter: list it in text/node-events/flag/filter", k)
	}
	for w := range inHelp {
		assert.True(t, slices.Contains(kinds, w), "the help of the event filter lists %s, no event kind", w)
	}
}
