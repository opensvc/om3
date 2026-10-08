package omcmd

import (
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDaemonUnreachable(t *testing.T) {
	_, dialErr := net.Dial("unix", filepath.Join(t.TempDir(), "http.sock"))
	// The http client wraps the dial error of its transport.
	unreachable := &url.Error{Op: "Get", URL: "http://localhost/api/cluster/status", Err: dialErr}
	assert.True(t, isDaemonUnreachable(unreachable), "no daemon listening")

	denied := errors.New("unexpected get daemon status code 403 Forbidden")
	assert.False(t, isDaemonUnreachable(denied), "a daemon denial")
	assert.False(t, isDaemonUnreachable(nil), "no error")
}
