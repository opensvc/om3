package rescontainerocibase

import (
	"net/netip"

	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
)

// nodeNameservers returns the nameservers of the resolver of the node, and
// is replaced by the tests.
var nodeNameservers = netwarden.NodeNameservers

// resolvConf returns the resolver om gives the container, zero when no
// nameserver is named: the container then keeps the resolver its engine
// gives it.
//
// The nameservers are the ones of the networks the addresses of the
// container are on, else the ones of the cluster, then the ones the dns
// keyword adds. The search list is the domain of the object and each of its
// parents, preceded by the domains the dns_search keyword adds.
//
// It is the one place saying which nameservers a container has: the file
// mounted in the container is written from it, and its network filter opens
// what it names.
func (t *BT) resolvConf() rescontainer.ResolvConf {
	nameservers := t.DNS
	if l := t.networkNameservers(); len(l) > 0 {
		nameservers = l
	}
	return rescontainer.ResolvConf{
		Nameservers: rescontainer.Nameservers(nameservers, t.DNSExtra),
		Searches:    rescontainer.SearchDomains(t.ObjectDomain, t.DNSSearch),
		Options:     rescontainer.ResolvConfOptions,
	}
}

// netNameservers returns the nameservers the container resolves through,
// which its network filter allows without a rule and its dns rule names.
//
// They are the ones om writes in its resolv.conf, the ones a resolver reads
// and no more. A container om writes no resolver for keeps the one of its
// engine, the resolver of the node on the host network, and what the engine
// makes of it elsewhere: the nameservers of the node are the ones known.
func (t *BT) netNameservers() []netip.Addr {
	resolvConf := t.resolvConf()
	if resolvConf.IsZero() {
		return nodeNameservers()
	}
	return netwarden.ParseNameservers(resolvConf.WrittenNameservers())
}
