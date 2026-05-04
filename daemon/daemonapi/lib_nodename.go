package daemonapi

import "github.com/opensvc/om3/v3/daemon/api"

// parseNodename supports "_" and "localhost" as our local host name aliases.
func (a *DaemonAPI) parseNodename(s string) string {
	switch s {
	case api.AliasLocalhost, api.AliasShortLocalhost:
		return a.localhost
	default:
		return s
	}
}
