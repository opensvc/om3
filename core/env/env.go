package env

import (
	"fmt"
	"os"
	"strings"
)

type (
	ActionOrigin string
)

var (
	ActionOriginVar                          = "OSVC_ACTION_ORIGIN"
	ActionOriginUser            ActionOrigin = "user"
	ActionOriginDaemonAPI       ActionOrigin = "daemon/api"
	ActionOriginDaemonMonitor   ActionOrigin = "daemon/monitor"
	ActionOriginDaemonScheduler ActionOrigin = "daemon/scheduler"

	NameVar      = "OSVC_NAME"
	NamespaceVar = "OSVC_NAMESPACE"
	KindVar      = "OSVC_KIND"
	ContextVar   = "OSVC_CONTEXT"

	NoLogFileVar = "OSVC_NO_LOG_FILE"
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

// Var returns the arg to pass to environment variable setter functions to hint
// the called CRM command was launched from a daemon policy.
func (t ActionOrigin) Var() string {
	var buff strings.Builder
	buff.WriteString(ActionOriginVar)
	buff.WriteString("=")
	buff.WriteString(string(t))
	return buff.String()
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

func NoLogFile() bool {
	return os.Getenv(NoLogFileVar) == "1"
}
func NoLogFileSetenvArg() string {
	return fmt.Sprintf("%s=1", NoLogFileVar)
}
