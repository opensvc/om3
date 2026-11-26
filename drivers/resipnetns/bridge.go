//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/opensvc/om3/v3/core/actionrollback"
)

func (t *T) startBridge(ctx context.Context) error {
	if t.onL2() {
		if err := t.checkL2Bridge(); err != nil {
			return err
		}
	}
	pid, err := t.getNSPID(ctx)
	if err != nil {
		return err
	}
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	if netns != nil {
		defer func() {
			if err := netns.Close(); err != nil {
				t.Log().Warnf("netns close: %s", err)
			}
		}()
	}
	guestDev, exists, err := t.guestDevOrigin(netns)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("device already exists")
		if t.onL2() {
			// A port taken off the bridge, as by a network torn down
			// and set up again: plugged back, as for no address.
			if err := t.replugL2(netns, guestDev); err != nil {
				return err
			}
		}
	} else if !t.hasNSDev(netns) {
		hostDev := formatHostDevName(guestDev, pid)

		mtu, err := t.devMTU()
		if err != nil {
			return err
		}
		tmpGuestDev := fmt.Sprintf("v%spg%d", guestDev, pid)
		if err := t.makeVethPair(hostDev, tmpGuestDev, mtu); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return t.linkDel(hostDev)
		})
		if t.onL2() {
			// The node holds no address on a layer 2 segment, and
			// takes no route advertised there: see makeL2Pair.
			if err := disableIPv6(hostDev); err != nil {
				return err
			}
		} else if addr := t.ipaddr(); addr != nil && addr.To4() == nil {
			if err := t.sysctlDisableIPV6RA(hostDev); err != nil {
				return err
			}
		}
		if err := t.linkSetNsPidAndNameAndUp(tmpGuestDev, pid, guestDev); err != nil {
			t.linkDel(tmpGuestDev)
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return t.linkDelIn(guestDev, netns.Path())
		})
		if err := t.linkSetMacIn(guestDev, t.macAddr(), netns.Path()); err != nil {
			return err
		}
		if err := t.linkSetMaster(hostDev, t.Dev); err != nil {
			t.linkDel(tmpGuestDev)
			return err
		}
		waitBridgePortForwarding(t, hostDev)
	}
	if t.onL2() && !exists {
		// An address the interface holds already is the resource's own.
		if err := t.checkL2Duplicate(netns, guestDev); err != nil {
			return err
		}
	}
	if err := t.startIP(ctx, netns, guestDev); err != nil {
		return err
	}
	if t.onL2() {
		// An ipv6 address another host of the segment holds: the
		// rollback of startIP removes it.
		if err := t.waitDAD(netns, guestDev); err != nil {
			return err
		}
	}
	if err := t.startRoutes(ctx, netns, guestDev); err != nil {
		return err
	}
	if err := t.startRoutesDel(ctx, netns, guestDev); err != nil {
		return err
	}
	if err := t.startSourceRoute(netns, guestDev); err != nil {
		return err
	}
	if err := t.startARP(netns, guestDev); err != nil {
		return err
	}
	return nil
}

// replugL2 makes the other end of guestDev on the node a port of the bridge
// of the layer 2 network again, when taken off it, and waits for the port to
// forward, as a port made by the start: the announces and the duplicate
// address detection that follow would be dropped by a port of a bridge
// running stp, still listening or learning.
func (t *T) replugL2(netns ns.NetNS, guestDev string) error {
	link, err := l2GuestLinkByName(netns, guestDev)
	if err != nil {
		return err
	}
	if link == nil || link.peer == "" || link.master == t.Dev {
		return nil
	}
	t.Log().Infof("%s is in the namespace, and its peer %s is not a port of %s: plug it back", guestDev, link.peer, t.Dev)
	if err := t.linkSetMaster(link.peer, t.Dev); err != nil {
		return err
	}
	waitBridgePortForwarding(t, link.peer)
	return nil
}

func (t *T) stopBridge(ctx context.Context) error {
	pid, err := t.getNSPID(ctx)
	if err != nil {
		return err
	}
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	if netns != nil {
		defer netns.Close()
	}

	guestDev := ""
	if netns != nil {
		if guestDev, err = t.curGuestDev(netns); err != nil {
			return err
		}
	}
	var hostDev string
	if guestDev != "" {
		hostDev = fmt.Sprintf("v%spl%d", guestDev, pid)
	} else if t.NSDev != "" {
		hostDev = fmt.Sprintf("v%spl%d", t.NSDev, pid)
	} else {
		return nil
	}

	if netns != nil {
		if err := t.stopIP(netns, guestDev); err != nil {
			return err
		}
		if err := t.stopLinkIn(netns, guestDev); err != nil {
			switch {
			case errors.Is(err, ErrLinkNotFound):
				// ignore, let del host dev be tried
			case errors.Is(err, ErrLinkInUse):
				return nil
			default:
				return err
			}
		}
	}
	if err := t.stopLink(hostDev); err != nil {
		if errors.Is(err, ErrLinkNotFound) {
			t.Log().Infof("link %s is already deleted", hostDev)
			return nil
		}
		return err
	}
	return nil
}

// bridgePortForwardingTimeout bounds the wait of a port of the bridge to
// forward: twice the default forward delay of a bridge running stp, and more.
const bridgePortForwardingTimeout = 35 * time.Second

// waitBridgePortForwarding is waitPortForwardingOf, a variable for the tests:
// the sysfs of a network namespace of a test is not the one it reads.
var waitBridgePortForwarding = (*T).waitPortForwardingOf

// waitPortForwardingOf waits for the port of the bridge the host end of
// the veth pair is, to forward: the gratuitous arp a start sends would be
// dropped by a port still listening or learning, as one of a bridge running
// stp is for its forward delay. A port of a bridge not running stp forwards
// at once. A port blocking or disabled, or a bridge not telling its port
// state, as an openvswitch one, is not waited for.
func (t *T) waitPortForwardingOf(hostDev string) {
	p := filepath.Join("/sys/class/net", hostDev, "brport", "state")
	read := func() (string, error) {
		b, err := os.ReadFile(p)
		return strings.TrimSpace(string(b)), err
	}
	state, waited := waitPortForwarding(read, bridgePortForwardingTimeout, 100*time.Millisecond)
	switch {
	case state == brStateForwarding && waited >= time.Second:
		t.Log().Infof("bridge port %s forwards after %s", hostDev, waited.Round(time.Millisecond))
	case state == brStateListening || state == brStateLearning:
		t.Log().Warnf("bridge port %s does not forward after %s, state %s: announce all the same", hostDev, waited.Round(time.Second), state)
	}
}

// The states of a port of a linux bridge, as /sys/class/net/<dev>/brport/state
// says them.
const (
	brStateListening  = "1"
	brStateLearning   = "2"
	brStateForwarding = "3"
)

// waitPortForwarding reads the state of a bridge port every poll until it
// forwards, leaves the listening and learning states, or timeout, and returns
// the last state read, empty when not read, and how long it waited.
func waitPortForwarding(read func() (string, error), timeout, poll time.Duration) (string, time.Duration) {
	begin := time.Now()
	for {
		state, err := read()
		if err != nil {
			return "", time.Since(begin)
		}
		if state != brStateListening && state != brStateLearning {
			return state, time.Since(begin)
		}
		if time.Since(begin) >= timeout {
			return state, time.Since(begin)
		}
		time.Sleep(poll)
	}
}
