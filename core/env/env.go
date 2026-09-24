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

	// JoinTokenVar is the environment variable the "cluster join" command
	// reads the join token from when --token is not set. The daemon uses it
	// to hand a token to the join it forks, so the token never appears in the
	// process command line, which any user can read.
	JoinTokenVar = "OSVC_JOIN_TOKEN"

	// CredentialVar is the environment variable the "cluster leave" command
	// reads the <username>:<password> of the user to create on the leaving
	// node from. The daemon uses it to hand a credential to the leave it
	// forks. There is no flag carrying the value itself, for the same reason
	// the join token has none: the process command line is world readable.
	CredentialVar = "OSVC_CREDENTIAL"

	// CollectorCredentialVar is the environment variable the collector
	// registration commands read the <username>:<password> of the collector
	// user from when --credential is not set. It is separate from
	// CredentialVar because it names a user of the collector, not of the
	// cluster, and an operator can hold one without the other.
	CollectorCredentialVar = "OSVC_COLLECTOR_CREDENTIAL"
)

// HasDaemonOrigin returns true if the environment variable OSVC_ACTION_ORIGIN
// is set to one of the daemon origins: "daemon/monitor", "daemon/api" or
// "daemon/scheduler". The opensvc daemon sets this variable on every command
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
// is set to "daemon/monitor". The opensvc daemon sets this variable on every command
// it executes.
func HasDaemonMonitorOrigin() bool {
	switch Origin() {
	case ActionOriginDaemonMonitor:
		return true
	default:
		return false
	}
}

// HasDaemonSchedulerOrigin returns true if the environment variable
// OSVC_ACTION_ORIGIN is set to "daemon/scheduler", which the daemon sets on
// the commands its scheduler runs.
func HasDaemonSchedulerOrigin() bool {
	switch Origin() {
	case ActionOriginDaemonScheduler:
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
