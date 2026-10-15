package resiphost

import (
	"context"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/util/ping"
)

// ProvisionConflicts says the address answers already, from another host or
// another object of this one: the provision would bring it up a second time.
// An address still to draw from a network is not checked: it is drawn free.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	ip := t.ipaddr()
	if ip == nil {
		return nil
	}
	if alive, err := ping.Ping(ip.String(), 5*time.Second); err == nil && alive {
		return []string{fmt.Sprintf("%s answers", ip)}
	}
	return nil
}
