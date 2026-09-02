package commoncmd

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/client"
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

// errReader is a body that cannot be read, as the body of a response
// whose caller already read and closed it.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("http2: response body closed") }

func (errReader) Close() error { return nil }

// TestDoNodeToleratesAConsumedBody covers the commands whose api call
// reads the response itself and hands the response over for the status
// check: reading it again fails, and that must not fail the action.
func TestDoNodeToleratesAConsumedBody(t *testing.T) {
	for name, tc := range map[string]struct {
		statusCode int
		hasError   bool
	}{
		"a successful action": {http.StatusOK, false},
		"a refused action":    {http.StatusBadRequest, true},
	} {
		t.Run(name, func(t *testing.T) {
			sub := CmdDaemonSubAction{}
			err := sub.doNode(context.Background(), nil, "node1",
				func(context.Context, *client.T, string) (*http.Response, error) {
					return &http.Response{StatusCode: tc.statusCode, Body: errReader{}}, nil
				})
			if tc.hasError {
				assert.Error(t, err, "a refused action is still refused")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
