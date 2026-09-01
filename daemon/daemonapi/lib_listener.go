package daemonapi

import (
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
)

// listenerName returns the name of the listener an action addresses.
//
// The action is published as a message the listener subscribes to by
// name, so a name no listener answers to is accepted, queued and acted
// on by nobody. The listeners are named the same here, in the audit
// subsystem list and in their own subscriptions, from daemonenv.
func listenerName(name api.InPathListenerName) (string, error) {
	s := string(name)
	if !slices.Contains(daemonenv.ListenerNames, s) {
		return "", fmt.Errorf("unknown listener: %s, expected one of %s", s, strings.Join(daemonenv.ListenerNames, ", "))
	}
	return s, nil
}
