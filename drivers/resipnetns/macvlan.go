//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/opensvc/om3/core/actionrollback"
)

func (t *T) startMACVLAN(ctx context.Context) error {
	pid, err := t.getNSPID(ctx)
	if err != nil {
		return err
	}
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	defer netns.Close()

	guestDev, err := t.guestDev(netns)
	if err != nil {
		return err
	}

	if err := t.startMACVLANDev(ctx, netns, pid, guestDev); err != nil {
		return err
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

func (t *T) startMACVLANDev(ctx context.Context, netns ns.NetNS, pid int, dev string) error {
	if t.hasNSDev(netns) {
		return nil
	}
	mtu, err := t.devMTU()
	if err != nil {
		return err
	}
	tmpDev := fmt.Sprintf("ph%d%s", pid, dev)
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
	if t.makeMACVLANDev(t.IPDev, tmpDev, mtu); err != nil {
		return err
	}
	if err := t.linkSetNsPidAndNameAndUp(tmpDev, pid, dev); err != nil {
		t.linkDel(tmpDev)
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.linkDelIn(dev, netns.Path())
	})
	return nil
}

func (t *T) stopMACVLAN(ctx context.Context) error {
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
