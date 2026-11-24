package rescontainerkvm

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/core/resource"
)

// Info implements the resource infoer: the uuid of the domain, as its
// persistent definition says it, so known while the domain is shut off. The
// guest sees it as its SMBIOS system uuid, and the collector links the
// guest node to this resource by it.
//
// A domain not defined, or a definition without uuid, adds no key.
func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	keys := resource.InfoKeys{}
	b, err := t.inactiveDefinition(ctx)
	if err != nil {
		t.Log().Debugf("info: %s", err)
		return keys, nil
	}
	if u := domainUUID(b); u != "" {
		keys = append(keys, resource.InfoKey{Key: "uuid", Value: strings.ToLower(u)})
	}
	return keys, nil
}
