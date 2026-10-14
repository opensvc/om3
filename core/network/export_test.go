package network

// CaptureFirewall replaces the firewall setup of Setup by capture, for the
// tests of the package network_test, which hold the network drivers, and
// returns what restores it.
func CaptureFirewall(capture func([]Networker)) (restore func()) {
	prev := setupFirewall
	setupFirewall = func(_ logger, nws []Networker) error {
		capture(nws)
		return nil
	}
	return func() { setupFirewall = prev }
}
