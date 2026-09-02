package oxcmd

import (
	"github.com/opensvc/om3/v3/daemon/api"
)

// hasEvalError returns true if at least one item could not be evaluated. The
// daemon reports those per item when the whole configuration is evaluated, so
// the ERROR column is only added to the default output when it has something
// to show.
func hasEvalError(items api.KeywordItems) bool {
	for _, item := range items {
		if item.Error != nil {
			return true
		}
	}
	return false
}
