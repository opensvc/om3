package network

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	// Teardowner is the interface of the networks undoing on the node what
	// their setup made, their configuration removed.
	Teardowner interface {
		Teardown() error
	}
)

// appliedConsumer is the name the network setup records the configuration it
// applied under.
const appliedConsumer = "network"

// teardownRemoved undoes the setup of the networks the configuration last
// applied holds and the current one, whose networks are nws, does not, then
// records the current one as applied.
//
// The configuration last applied is the one recorded, not the one the daemon
// held before a change: a network removed while the daemon was down, or by an
// edit of the file, is torn down at the next setup all the same, from the
// configuration it was set up with, its section gone from the current one.
//
// A network failing its teardown leaves the record as it was, so the next
// setup tears it down again.
func teardownRemoved(n *object.Node, nws []Networker) error {
	applied, err := object.LoadApplied(appliedConsumer, object.WithLogger(n.Log()))
	if err != nil {
		return fmt.Errorf("load the configuration last applied: %w", err)
	}
	if applied != nil {
		errs := make([]error, 0)
		for _, nw := range removed(Networks(applied), nws) {
			if err := teardownNetwork(applied, nw); err != nil {
				nw.Log().Errorf("network teardown: %s", err)
				errs = append(errs, fmt.Errorf("%s: %w", nw.Name(), err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("teardown: %w", errors.Join(errs...))
		}
	}
	return n.SaveApplied(appliedConsumer)
}

// removed returns the networks of former the networks of current do not name.
func removed(former, current []Networker) []Networker {
	names := make([]string, len(current))
	for i, nw := range current {
		names[i] = nw.Name()
	}
	l := make([]Networker, 0)
	for _, nw := range former {
		if !slices.Contains(names, nw.Name()) {
			l = append(l, nw)
		}
	}
	return l
}

// teardownNetwork undoes the setup of the network nw, of the node n as
// configured when nw was set up: what its driver made, and its cni
// configuration. Its firewall rules are not, the ruleset of the networks being
// rewritten whole by the setup.
func teardownNetwork(n *object.Node, nw Networker) error {
	nw.Log().Infof("network teardown: its configuration is removed")
	warnUsers(nw)
	if i, ok := nw.(Teardowner); ok {
		if err := i.Teardown(); err != nil {
			return err
		}
	}
	if _, ok := nw.(CNIer); !ok {
		return nil
	}
	dir, err := n.CNIConfig()
	if err != nil {
		return err
	}
	p := CNIConfigFile(dir, nw)
	if err := os.Remove(p); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	nw.Log().Infof("cni %s removed", p)
	return nil
}

// warnUsers says the objects of the node holding an address of the network
// torn down, which lose it: the teardown is the operator's decision, made
// when they removed the network.
func warnUsers(nw Networker) {
	ipnet, err := nw.IPNet()
	if err != nil || ipnet == nil {
		return
	}
	reservations, err := localReservations()
	if err != nil {
		return
	}
	paths := make([]string, 0)
	for _, r := range reservations {
		if !ipnet.Contains(r.IP) {
			continue
		}
		p, ok := ipam.PathOfKey(r.Key)
		if !ok || slices.Contains(paths, p.String()) {
			continue
		}
		paths = append(paths, p.String())
	}
	if len(paths) > 0 {
		nw.Log().Warnf("network teardown: %s still hold an address of this network, which they lose", strings.Join(paths, ", "))
	}
}
