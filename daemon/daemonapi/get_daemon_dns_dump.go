package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/dns"
)

// GetDaemonDNSDump returns the DNS zone content.
func (a *DaemonApi) GetDaemonDNSDump(w http.ResponseWriter, r *http.Request) {
	zone := dns.GetZone()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(zone)
	w.WriteHeader(http.StatusOK)
}
