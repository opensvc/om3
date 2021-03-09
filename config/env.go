package config

import "os"

// HasDaemonOrigin returns true if the environment variable OSVC_ACTION_ORIGIN
// is set to true. The opensvc daemon sets this variable on every command
// it executes.
func HasDaemonOrigin() bool {
	return os.Getenv("OSVC_ACTION_ORIGIN") == "daemon"
}

// EnvNamespace returns the namespace filter forced via the OSVC_NAMESPACE environment
// variable.
func EnvNamespace() string {
	return os.Getenv("OSVC_NAMESPACE")
}

// EnvKind returns the object kind filter forced via the OSVC_NAMESPACE environment
// variable.
func EnvKind() string {
	return os.Getenv("OSVC_KIND")
}
