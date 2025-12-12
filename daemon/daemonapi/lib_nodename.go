package daemonapi

import "github.com/opensvc/om3/v3/daemon/api"

func (a *DaemonAPI) parseNodename(s string) string {
	switch s {
	case api.AliasLocalhost, api.AliasShortLocalhost:
		return a.localhost
	default:
		return s
	}
}
