//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/opensvc/om3/core/actionrollback"
)

func (t *T) IPVLANMode() (string, error) {
	switch t.Mode {
	case "ipvlan-l2":
		return "l2", nil
	case "ipvlan-l3":
		return "l3", nil
	case "ipvlan-l3s":
		return "l3s", nil
	default:
		return "", fmt.Errorf("unsupported mode: %s", t.Mode)
	}
}

func (t *T) startIPVLANDev(ctx context.Context, netns ns.NetNS, pid int, dev string) error {
	tmpDev := fmt.Sprintf("ph%d%s", pid, dev)
	mode, err := t.IPVLANMode()
	if err != nil {
		return err
	}
	mtu, err := t.devMTU()
	if err != nil {
		return err
	}
	if v, err := t.hasLink(tmpDev); err != nil {
		return err
	} else if v {
		if err := t.linkDel(tmpDev); err != nil {
			return err
		}
	}
	if v, err := t.hasLinkIn(dev, netns.Path()); err != nil {
		return err
	} else if v {
		return nil
	}
	if err := t.makeIPVLANDev(t.Dev, tmpDev, mtu, mode); err != nil {
		return err
	}
	if err := t.linkSetNsPidAndNameAndUp(tmpDev, pid, dev); err != nil {
		t.linkDel(tmpDev)
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.linkDelIn(dev, netns.Path())
	})
	return nil
}

func (t *T) startIPVLAN(ctx context.Context) error {
	pid, err := t.getNSPID(ctx)
	if err != nil {
		return err
	}
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	defer netns.Close()

	guestDev, exists, err := t.guestDevOrigin(netns)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("device already exists")
	} else if !t.hasNSDev(netns) {
		if err := t.startIPVLANDev(ctx, netns, pid, guestDev); err != nil {
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
	netns, err := t.getNS(ctx)
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
	return nil
}
