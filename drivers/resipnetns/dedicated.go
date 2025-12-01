//go:build linux

package resipnetns

import (
	"context"
	"strings"
)

func (t *T) startDedicated(ctx context.Context) error {
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
		if v, err := t.hasLink(t.Dev); err != nil {
			return err
		} else if !v {
			t.Log().Infof("dev %s not found in the host ns... may already be in netns %d", t.Dev, pid)
		} else if err := t.linkSetNsPidAndNameAndUp(t.Dev, pid, guestDev); err != nil {
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

func (t *T) stopDedicated(ctx context.Context) error {
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
	if guestDev == "" {
		if t.NSDev != "" {
			guestDev = t.NSDev
		} else {
			return nil
		}
	}
	if err := t.stopIP(netns, guestDev); err != nil {
		return err
	}
	if v, err := t.hasLinkIn(guestDev, netns.Path()); err != nil {
		return err
	} else if !v {
		return nil
	}
	if addrs, err := t.getAddrStringsIn(guestDev, netns); err != nil {
		return err
	} else if len(addrs) > 0 {
		t.Log().Infof("preserve nsdev %s, in use by %s", guestDev, strings.Join(addrs, " "))
		return ErrLinkInUse
	}
	return t.linkSetNsPidAndNameIn(guestDev, 1, t.Dev, netns.Path())
}
