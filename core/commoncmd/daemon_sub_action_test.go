package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProblemString covers what a failed daemon sub-action reports: the
// body says why, and reporting only the status code makes the caller
// guess what a 400 was about.
func TestProblemString(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"a title and a detail": {
			`{"title":"Invalid body","detail":"the daemon log is not emitted below the info level","status":400}`,
			"Invalid body: the daemon log is not emitted below the info level",
		},
		"a title alone":     {`{"title":"Forbidden","status":403}`, "Forbidden"},
		"a detail alone":    {`{"detail":"missing 'level' field","status":400}`, "missing 'level' field"},
		"an empty document": {`{}`, ""},
		"something else":    {`not json`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, problemString([]byte(tc.body)))
		})
	}
}
