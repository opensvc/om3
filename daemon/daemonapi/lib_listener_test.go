package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
)

// TestListenerNamesAgreeWithTheApiEnum pins the two lists that have to
// hold the same names: the one the listeners subscribe and audit
// themselves with, and the one the api spec publishes. A name in one and
// not the other is an action accepted by the api and answered by no
// listener, or a listener no client can address.
func TestListenerNamesAgreeWithTheApiEnum(t *testing.T) {
	for _, name := range daemonenv.ListenerNames {
		assert.Truef(t, api.DaemonListenerName(name).Valid(), "%s is not in the api enum", name)
	}
	for _, name := range []api.DaemonListenerName{api.ApiUx, api.ApiInet} {
		_, err := listenerName(api.InPathListenerName(name))
		assert.NoErrorf(t, err, "%s is in the api enum but is no listener", name)
	}
}

func TestListenerNameRefusesWhatNoListenerAnswersTo(t *testing.T) {
	// The name the actions took before they were aligned with the audit
	// subsystem names is the one to refuse loudest: it used to be
	// accepted, queued, and acted on by nobody.
	for _, name := range []string{"", "http-inet", "lsnr-http-inet", "api", "api.*"} {
		_, err := listenerName(api.InPathListenerName(name))
		require.Errorf(t, err, "%s must not be accepted", name)
		assert.Contains(t, err.Error(), daemonenv.ListenerNameInet, "the error must name the listeners that exist")
	}
}
