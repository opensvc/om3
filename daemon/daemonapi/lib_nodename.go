package daemonapi

const (
	aliasLocalhost      = "localhost"
	aliasShortLocalhost = "_"
)

func (a *DaemonAPI) parseNodename(s string) string {
	switch s {
	case aliasLocalhost, aliasShortLocalhost:
		return a.localhost
	default:
		return s
	}
}
