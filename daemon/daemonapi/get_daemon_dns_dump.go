package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/dns"
)

// GetDaemonDNSDump returns the DNS zone content.
func (a *DaemonApi) GetDaemonDNSDump(w http.ResponseWriter, r *http.Request) {
	zone := dns.GetZone()
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(zone)
}
