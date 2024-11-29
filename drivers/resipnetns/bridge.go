//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/core/actionrollback"
)

func (t *T) startBridgePort(ctx context.Context, dev string) error {
	masterLink, err := netlink.LinkByName(t.IPDev)
	if err != nil {
		return fmt.Errorf("%s: %w", t.IPDev, err)
	}
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return fmt.Errorf("%s: %w", dev, err)
	}
	actionrollback.Register(ctx, func() error {
		return t.stopBridgePort(dev)
	})
	t.Log().Infof("set %s master %s", dev, t.IPDev)
	return netlink.LinkSetMaster(link, masterLink)
}

func (t *T) stopBridgePort(dev string) error {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return nil
	}
	t.Log().Infof("unset %s master %s", dev, t.IPDev)
	return netlink.LinkSetMaster(link, nil)
}

func (t *T) startBridge(ctx context.Context) error {
	pid, err := t.getNSPID()
	if err != nil {
		return err
	}
	netns, err := t.getNS()
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
	guestDev, err := t.guestDev(netns)
	if err != nil {
		return err
	}
	if !t.hasNSDev(netns) {
		hostDev := formatHostDevName(guestDev, pid)

		mtu, err := t.devMTU()
		if err != nil {
			return err
		}

		if err := t.startVEthPair(ctx, netns, hostDev, guestDev, mtu); err != nil {
			return err
		}
		if err := t.startBridgePort(ctx, hostDev); err != nil {
			return err
		}
	}
	if err := t.startIP(ctx, netns, guestDev); err != nil {
		return err
	}
	if err := t.startRoutes(ctx, netns, guestDev); err != nil {
		return err
	}
	if err := t.startRoutesDel(ctx, netns, guestDev); err != nil {
		return err
	}
	if err := t.startARP(netns, guestDev); err != nil {
		return err
	}
	return nil
}

func (t *T) stopBridge(ctx context.Context) error {
	var hostDev string

	pid, err := t.getNSPID()
	if err != nil {
		return err
	}
	netns, err := t.getNS()
	if err != nil {
		return err
	}
	if netns == nil {
		return nil
	}
	defer netns.Close()

	guestDev, err := t.curGuestDev(netns)
	if err != nil {
		return err
	}
	if guestDev != "" {
		hostDev = fmt.Sprintf("v%spl%d", guestDev, pid)
	} else if t.NSDev != "" {
		hostDev = fmt.Sprintf("v%spl%d", t.NSDev, pid)
	}

	if err := t.stopIP(netns, guestDev); err != nil {
		return err
	}
	if err := t.stopLink(netns, guestDev); err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		if errors.Is(err, ErrLinkInUse) {
			return nil
		}
		return err
	}
	if err := t.stopBridgePort(hostDev); err != nil {
		return err
	}
	if err := t.stopVEthPair(hostDev); err != nil {
		return err
	}
	return nil
}
