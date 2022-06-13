package env

import "os"

// HasDaemonOrigin returns true if the environment variable OSVC_ACTION_ORIGIN
// is set to "daemon". The opensvc daemon sets this variable on every command
// it executes.
func HasDaemonOrigin() bool {
	return os.Getenv("OSVC_ACTION_ORIGIN") == "daemon"
}

// Origin returns the action origin using a env var that the daemon sets when
// executing a CRM action. The only possible return values are "daemon" or "user".
func Origin() string {
	s := os.Getenv("OSVC_ACTION_ORIGIN")
	if s == "" {
		s = "user"
	}
	return s
}

// Namespace returns the namespace filter forced via the OSVC_NAMESPACE environment
// variable.
func Namespace() string {
	return os.Getenv("OSVC_NAMESPACE")
}

// Kind returns the object kind filter forced via the OSVC_NAMESPACE environment
// variable.
func Kind() string {
	return os.Getenv("OSVC_KIND")
}

// Context returns the identifier of a remote cluster endpoint and credentials
// configuration via the OSVC_CONTEXT variable.
func Context() string {
	return os.Getenv("OSVC_CONTEXT")
}
