package env

import "os"

// HasDaemonOrigin returns true if the environment variable OSVC_ACTION_ORIGIN
// is set to true. The opensvc daemon sets this variable on every command
// it executes.
func HasDaemonOrigin() bool {
	return os.Getenv("OSVC_ACTION_ORIGIN") == "daemon"
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
