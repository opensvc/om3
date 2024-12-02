//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/util/command"
	"github.com/rs/zerolog"
	"github.com/vishvananda/netlink"
)

func (t *T) startMACVLAN(ctx context.Context) error {
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
		if err := t.startMACVLANDev(ctx, netns, pid, guestDev, mtu); err != nil {
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

func (t *T) startMACVLANDev(ctx context.Context, netns ns.NetNS, pid int, dev string, mtu int) error {
	tmpDev := fmt.Sprintf("ph%d%s", pid, dev)
	if _, err := netlink.LinkByName(tmpDev); err == nil {
		return fmt.Errorf("%s exists, should not", tmpDev)
	}
	args := []string{"ip", "link", "add", "link", t.IPDev, "dev", tmpDev, "mtu", fmt.Sprint(mtu), "type", "macvlan", "mode", "bridge"}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := t.linkSetNsPid(tmpDev, pid); err != nil {
		t.linkDel(tmpDev)
		return err
	}
	if err := t.linkSetNameIn(tmpDev, dev, netns.Path()); err != nil {
		t.linkDelIn(tmpDev, netns.Path())
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.linkDelIn(dev, netns.Path())
	})
	if err := t.linkSetUpIn(dev, netns.Path()); err != nil {
		return err
	}
	return nil
}

func (t *T) stopMACVLAN(ctx context.Context) error {
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
	return nil
}
