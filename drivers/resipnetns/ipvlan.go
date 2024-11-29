//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/core/actionrollback"
)

func (t T) IPVLANMode() (netlink.IPVlanMode, error) {
	switch t.Mode {
	case "ipvlan-l2":
		return netlink.IPVLAN_MODE_L2, nil
	case "ipvlan-l3":
		return netlink.IPVLAN_MODE_L3, nil
	case "ipvlan-l3s":
		return netlink.IPVLAN_MODE_L3S, nil
	default:
		return 0, fmt.Errorf("unsupported mode: %s", t.Mode)
	}
}

func (t *T) startIPVLANDev(ctx context.Context, netns ns.NetNS, pid int, dev string, mtu int) error {
	tmpDev := fmt.Sprintf("ph%d%s", pid, dev)
	parentLink, err := netlink.LinkByName(t.IPDev)
	if err != nil {
		return fmt.Errorf("%s: %w", t.IPDev, err)
	}
	if _, err := netlink.LinkByName(tmpDev); err == nil {
		return fmt.Errorf("%s exists, should not", tmpDev)
	}
	mode, err := t.IPVLANMode()
	if err != nil {
		return err
	}
	t.Log().Infof("ip link add link %s dev %s mtu %d mode %s mtu %d", t.IPDev, tmpDev, mtu, t.Mode, mtu)
	link := &netlink.IPVlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        tmpDev,
			Flags:       net.FlagUp,
			MTU:         mtu,
			ParentIndex: parentLink.Attrs().Index,
		},
		Mode: mode,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return err
	}
	t.Log().Infof("ip link %s set netns %d", tmpDev, pid)
	if err := netlink.LinkSetNsPid(link, pid); err != nil {
		return err
	} else {
		netlink.LinkDel(link)
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		t.Log().Infof("nsenter --net=%s ip link set %s name %s", netns.Path(), tmpDev, dev)
		if err := netlink.LinkSetName(link, dev); err != nil {
			return err
		}
		t.Log().Infof("nsenter --net=%s ip link set %s up", netns.Path(), dev)
		if err := netlink.LinkSetUp(link); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.stopIPVLANDev(netns, dev)
	})
	return nil
}

func (t *T) stopIPVLANDev(netns ns.NetNS, dev string) error {
	if err := netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(dev)
		if err != nil {
			t.Log().Infof("container dev %s already deleted", dev)
			return nil
		}
		t.Log().Infof("nsenter --net=%s ip link del dev %s", netns.Path(), dev)
		return netlink.LinkDel(link)
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) startIPVLAN(ctx context.Context) error {
	pid, err := t.getNSPID()
	if err != nil {
		return err
	}
	netns, err := t.getNS()
	if err != nil {
		return err
	}
	defer netns.Close()

	guestDev, err := t.guestDev(netns)
	if err != nil {
		return err
	}

	if !t.hasNSDev(netns) {
		mtu, err := t.devMTU()
		if err != nil {
			return err
		}

		if err := t.startIPVLANDev(ctx, netns, pid, guestDev, mtu); err != nil {
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

func (t *T) stopIPVLAN(ctx context.Context) error {
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
	if err := t.stopIPVLANDev(netns, guestDev); err != nil {
		return err
	}
	return nil
}
