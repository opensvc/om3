package daemonapi

import (
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/netwarden"
)

// GetNetworkAddress returns the addresses the resources of the objects named,
// and of the objects of the namespaces named, publish in their status, the
// instances of all the nodes together: what the rules of net_rules
// naming an object or a namespace resolve to.
//
// Each object is told with its id and the version of its configuration,
// where known, and each address with the ones of the object publishing it:
// the start records which object published an address, not its path alone,
// for the record of a delete to be weighed against it, and a path names two
// objects while a node still tells its instance of the one deleted beside
// the one created again.
//
// A driver starting a resource with such rules asks it of the daemon of its
// node, the only one to know the objects of the other nodes. The addresses
// say how the objects of any namespace are reached: the root and inspector
// grants read them, as the addresses of the networks.
func (a *DaemonAPI) GetNetworkAddress(ctx echo.Context, params api.GetNetworkAddressParams) error {
	if v, err := assertInspector(ctx); !v {
		return err
	}
	var (
		objects    []naming.Path
		namespaces []string
	)
	if params.Object != nil {
		for _, s := range *params.Object {
			p, err := naming.ParsePath(s)
			if err != nil {
				return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "object %s: %s", s, err)
			}
			objects = append(objects, p)
		}
	}
	if params.Namespace != nil {
		namespaces = *params.Namespace
	}
	data := netwarden.Objects(objects, namespaces)
	identities := netwarden.Identities(data)
	paths := make([]string, 0, len(data))
	for s := range data {
		paths = append(paths, s)
	}
	slices.SortFunc(paths, strings.Compare)
	items := make(api.NetworkAddressItems, 0, len(paths))
	for _, s := range paths {
		resources := make(map[string][]string, len(data[s]))
		for rid, addrs := range data[s] {
			l := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				l = append(l, addr.String())
			}
			resources[rid] = l
		}
		item := api.NetworkAddress{Path: s, Resources: resources}
		if identity, ok := identities[s]; ok {
			if identity.ObjectID != "" {
				item.ObjectId = &identity.ObjectID
			}
			if !identity.Version.IsZero() {
				item.Version = &identity.Version
			}
			if identity.Publishers != nil {
				holders := holdersOf(identity.Publishers)
				item.Holders = &holders
			}
		}
		items = append(items, item)
	}
	return ctx.JSON(http.StatusOK, api.NetworkAddressList{Kind: "NetworkAddressList", Items: items})
}

// holdersOf returns the objects publishing the addresses given, each with
// its addresses, sorted, the objects in the order of their first address.
//
// The list is never nil, and so told when empty: an object told with no
// holder is one no address is known the object of, where the field left out
// is a daemon not telling the objects by address.
func holdersOf(publishers map[netip.Addr]netrule.Publisher) []api.NetworkAddressHolder {
	addrs := make([]netip.Addr, 0, len(publishers))
	for addr := range publishers {
		addrs = append(addrs, addr)
	}
	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })
	holders := make([]api.NetworkAddressHolder, 0)
	told := make([]netrule.Publisher, 0)
	for _, addr := range addrs {
		publisher := publishers[addr]
		i := slices.IndexFunc(told, func(o netrule.Publisher) bool {
			return publisher.ObjectID == o.ObjectID && publisher.Version.Equal(o.Version)
		})
		if i < 0 {
			holder := api.NetworkAddressHolder{Addresses: make([]string, 0, 1)}
			if publisher.ObjectID != "" {
				holder.ObjectId = &publisher.ObjectID
			}
			if !publisher.Version.IsZero() {
				holder.Version = &publisher.Version
			}
			holders, told = append(holders, holder), append(told, publisher)
			i = len(holders) - 1
		}
		holders[i].Addresses = append(holders[i].Addresses, addr.String())
	}
	return holders
}
