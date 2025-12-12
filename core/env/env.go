package env

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	ActionOrigin string
)

var (
	ActionOrchestrationIDVar                 = "OSVC_ACTION_ORCHESTRATION_ID"
	ActionOriginVar                          = "OSVC_ACTION_ORIGIN"
	ActionOriginUser            ActionOrigin = "user"
	ActionOriginDaemonAPI       ActionOrigin = "daemon/api"
	ActionOriginDaemonMonitor   ActionOrigin = "daemon/monitor"
	ActionOriginDaemonScheduler ActionOrigin = "daemon/scheduler"

	ParentSessionIDVar = "OSVC_PARENT_SESSION_UUID"
	NameVar            = "OSVC_NAME"
	NamespaceVar       = "OSVC_NAMESPACE"
	KindVar            = "OSVC_KIND"
	ContextVar         = "OSVC_CONTEXT"
)

// HasDaemonOrigin returns true if the environment variable OSVC_ACTION_ORIGIN
// is set to "daemon". The opensvc daemon sets this variable on every command
// it executes.
func HasDaemonOrigin() bool {
	switch Origin() {
	case ActionOriginDaemonMonitor, ActionOriginDaemonAPI, ActionOriginDaemonScheduler:
		return true
	default:
		return false
	}
}

// HasDaemonMonitorOrigin returns true if the environment variable OSVC_ACTION_ORIGIN
// is set to "daemon/imon". The opensvc daemon sets this variable on every command
// it executes.
func HasDaemonMonitorOrigin() bool {
	switch Origin() {
	case ActionOriginDaemonMonitor:
		return true
	default:
		return false
	}
}

// Origin returns the action origin using a env var that the daemon sets when
// executing a CRM action.
func Origin() ActionOrigin {
	s := os.Getenv(ActionOriginVar)
	if s == "" {
		return ActionOriginUser
	}
	return ActionOrigin(s)
}

// OriginSetenvArg returns the arg to pass to environment variable
// setter functions to hint the called CRM command was launched from a daemon
// policy.
func OriginSetenvArg(s ActionOrigin) string {
	return fmt.Sprintf("%s=%s", ActionOriginVar, s)
}

// ParentSessionIDSetenvArg returns the arg to pass to environment variable
// setter functions to hint the called CRM command was launched with a different
// session id.
func ParentSessionIDSetenvArg() string {
	return ParentSessionIDVar + "=" + xsession.ID.String()
}

// Namespace returns the namespace filter forced via the OSVC_NAMESPACE environment
// variable.
func Namespace() string {
	return os.Getenv(NamespaceVar)
}

// Kind returns the object kind filter forced via the OSVC_NAMESPACE environment
// variable.
func Kind() string {
	return os.Getenv(KindVar)
}

// Context returns the identifier of a remote cluster endpoint and credentials
// configuration via the OSVC_CONTEXT variable.
func Context() string {
	return os.Getenv(ContextVar)
}
