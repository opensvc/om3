package rescontainerkvm

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
)

// netKeywords is net_rules, beside the networks of the guest:
// their filter is applied on the interfaces om makes for them, by nftables,
// which knows the ports.
var netKeywords = netwarden.KeywordsOf(rescontainer.GroupNetwork.ID, netrule.BackendNetdev)

// netDevs returns the names of the interfaces om makes for the guest, one by
// network, which exist while its network is up.
func (t *T) netDevs() []string {
	key := ipam.Key(t.Path, t.RID())
	l := make([]string, 0, len(t.Network))
	for i := range t.Network {
		dev, _ := nicNames(key, i)
		l = append(l, dev)
	}
	return l
}

// netNameservers returns the nameservers the guest is told, the ones of the
// networks its interfaces are on, which the dns rule and the nameservers
// allowed without a rule name. A guest told none resolves through what it
// is configured with, which om does not know: none is opened.
func (t *T) netNameservers() []netip.Addr {
	l := make([]string, 0)
	nics, err := t.guestNICs(context.Background(), false)
	if err != nil {
		t.Log().Warnf("netwarden: the nameservers of the guest: %s", err)
	}
	for _, n := range nics {
		l = append(l, n.nameservers...)
	}
	return netwarden.ParseNameservers(l)
}

// netTarget is what the network filter of the guest is applied to: the
// interfaces om makes for it. The traffic of a guest is not the one of the
// qemu process, which the cgroup of its process group would filter: it
// leaves the node through the interfaces.
func (t *T) netTarget() netwarden.Target {
	return netwarden.Target{
		Path:    t.Path,
		Backend: netrule.BackendNetdev,
		Devs:    t.netDevs(),
		VarDir:  t.VarDir,
		Log:     t.Log(),
		DNS:     t.netNameservers,
	}
}

// NetRules implements netwarden.Driver.
func (t *T) NetRules() (netwarden.T, netwarden.Target) {
	return t.Net, t.netTarget()
}

// NetLiveIssue implements netwarden.Driver: what refuses a start its
// filter keeps one from being set under the running guest.
func (t *T) NetLiveIssue(context.Context) error {
	return t.netIssue()
}

// netIssue returns what keeps the network filter of the guest from being
// enforced, nil when nothing does, or when it has no rule.
//
// The filter is set on the interfaces om makes, the ones of the network
// keyword: an interface libvirt makes, as the ones of bridge or of a
// virt-install command, exists once the guest runs, unfiltered until then,
// and is not om's to name.
func (t *T) netIssue() error {
	if !t.Net.Enabled() {
		return nil
	}
	if len(t.Network) == 0 {
		return fmt.Errorf("netwarden: %s filters the interfaces om makes for the guest, the ones of the network keyword, and it names none", netwarden.Option)
	}
	return t.Net.Check(t.netTarget())
}

// netApply sets the network filter of the guest on its interfaces, once
// they are made and before they are plugged into their bridge. A guest with
// no rule has none to set: what an earlier configuration left was removed
// with its interfaces, see networkUp.
func (t *T) netApply() error {
	if !t.Net.Enabled() {
		return nil
	}
	_, err := t.Net.Apply(t.netTarget())
	return err
}

// netClear removes the network filter of the guest once its interfaces are
// gone. A failure is warned of: the guest stopped.
func (t *T) netClear() {
	if err := t.Net.Clear(t.netTarget()); err != nil {
		t.Log().Warnf("%s", err)
	}
}

// netStatus adds what the status of the running guest says of its network
// filter to its status log.
func (t *T) netStatus() {
	if err := t.netIssue(); err != nil && len(t.Network) == 0 {
		t.StatusLog().Warn("%s", err)
		return
	}
	infos, warnings := t.Net.Status(t.netTarget())
	for _, s := range infos {
		t.StatusLog().Info("%s", s)
	}
	for _, s := range warnings {
		t.StatusLog().Warn("%s", s)
	}
}
