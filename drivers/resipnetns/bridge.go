//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/actionrollback"
)

func (t *T) startBridge(ctx context.Context) error {
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
		tmpGuestDev := fmt.Sprintf("v%spg%d", guestDev, pid)
		if err := t.makeVethPair(hostDev, tmpGuestDev, mtu); err != nil {
			return err
		}
		actionrollback.Register(ctx, func() error {
			return t.linkDel(hostDev)
		})
		if err := t.sysctlDisableIPV6RA(hostDev); err != nil {
			return err
		}
		if err := t.linkSetMaster(hostDev, t.IPDev); err != nil {
			t.linkDel(tmpGuestDev)
			return err
		}
		if err := t.linkSetNsPidAndNameAndUp(tmpGuestDev, pid, guestDev); err != nil {
			t.linkDel(tmpGuestDev)
			return err
		}
		actionrollback.Register(ctx, func() error {
			return t.linkDelIn(guestDev, netns.Path())
		})
		if err := t.linkSetMacIn(guestDev, t.MacAddr, netns.Path()); err != nil {
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
