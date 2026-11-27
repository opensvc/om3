package rescontainerkvm

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/resource"
)

// Info implements the resource infoer: the uuid of the domain, as its
// persistent definition says it, so known while the domain is shut off. The
// guest sees it as its SMBIOS system uuid, and the collector links the
// guest node to this resource by it.
//
// A domain not defined, or a definition without uuid, adds no key. Another
// failure to read the definition is returned: the refresh then keeps the
// cached resource info, the uuid a wake needs included, instead of saving
// it without.
func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	keys := resource.InfoKeys{}
	b, err := t.inactiveDefinition(ctx)
	if isDomainNotFound(err) {
		t.Log().Debugf("info: %s", err)
		return keys, nil
	} else if err != nil {
		return keys, fmt.Errorf("info: %w", err)
	}
	if u := domainUUID(b); u != "" {
		keys = append(keys, resource.InfoKey{Key: "uuid", Value: strings.ToLower(u)})
	}
	return keys, nil
}

// isDomainNotFound tells whether err is virsh failing to find the domain,
// as dominfo is read for the domain state.
func isDomainNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "failed to get domain")
}
