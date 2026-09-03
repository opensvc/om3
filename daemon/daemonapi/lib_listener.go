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

// lifecycleListenerName returns the name of the listener a start, stop or
// restart addresses.
//
// The unix socket listener answers to none of the three: it lives as long as
// the daemon does, and the request asking for its restart travels through it.
// It used to be handed the message, log that it ignored it, and leave the
// caller told the action was queued.
func lifecycleListenerName(name api.InPathListenerName, action string) (string, error) {
	s, err := listenerName(name)
	if err != nil {
		return "", err
	}
	if !slices.Contains(daemonenv.LifecycleListenerNames, s) {
		return "", fmt.Errorf("%s has no %s of its own: it lives as long as the daemon does, expected one of %s",
			s, action, strings.Join(daemonenv.LifecycleListenerNames, ", "))
	}
	return s, nil
}
