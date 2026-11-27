package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/dns"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// dnsReaderRoles are the roles on a namespace that let a requester see the
// records of its objects in the zone.
var dnsReaderRoles = []rbac.Role{rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin}

// GetDaemonDNSDump returns the DNS zone content: the records of the objects
// of the namespaces the requester reads, guest, operator or admin, and the
// records of the cluster. Root and the inspector, which reads how the
// networks are set up, are given every one's. The addresses of a namespace
// are not told to a user reading none of its objects, as the status of a
// filter naming them only counts them.
func (a *DaemonAPI) GetDaemonDNSDump(ctx echo.Context, nodename string) error {
	sees, ok := dnsZoneSees(grantsFromContext(ctx))
	if !ok {
		return JSONForbiddenMissingRole(ctx, append(dnsReaderRoles, rbac.RoleInspector, rbac.RoleRoot)...)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		zone := dns.GetZoneOf(sees)
		resp := api.DNSRecordList{Kind: api.DNSRecordListKindDNSRecordList, Items: make(api.DNSZone, 0, len(zone))}
		for _, r := range zone {
			resp.Items = append(resp.Items, api.DNSRecord{Qname: r.Name, Qtype: r.Type, Ttl: r.TTL, Content: r.Content, DomainID: r.DomainID})
		}
		return ctx.JSON(http.StatusOK, resp)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetDaemonDNSDump(ctx.Request().Context(), nodename)
	})
}

// dnsZoneSees returns which namespaces the grants see the records of the
// objects of, every one when nil, and false when they see none.
func dnsZoneSees(grants rbac.Grants) (func(string) bool, bool) {
	switch {
	case seesNetworkSetup(grants), grants.HasRoleOn("", dnsReaderRoles...):
		return nil, true
	case len(grants.Namespaces(dnsReaderRoles...)) > 0:
		return func(namespace string) bool {
			return grants.AssertRoleOn(namespace, dnsReaderRoles...)
		}, true
	}
	return nil, false
}
