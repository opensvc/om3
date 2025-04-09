//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/util/command"
)

func (t *T) startOVSPort(ctx context.Context, dev string) error {
	args := []string{
		"--may-exist",
		"add-port", t.Dev, dev,
		fmt.Sprintf("vlan_mode=%s", t.VLANMode),
	}
	cmd := command.New(
		command.WithName("ovs-vsctl"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}

	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stopOVSPort(dev)
	})
	return nil
}

func (t *T) stopOVSPort(dev string) error {
	if dev == "" {
		return nil
	}
	return command.New(
		command.WithName("ovs-vsctl"),
		command.WithArgs([]string{"--if-exist", "del-port", t.Dev, dev}),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) startOVS(ctx context.Context) error {
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
		if addr := t.ipaddr(); addr != nil && addr.To4() == nil {
			if err := t.sysctlDisableIPV6RA(hostDev); err != nil {
				return err
			}
		}
		if err := t.linkSetNsPid(tmpGuestDev, pid); err != nil {
			t.linkDel(guestDev)
			return err
		}
		if err := t.linkSetNameIn(tmpGuestDev, guestDev, netns.Path()); err != nil {
			var errs error
			if err := t.linkDel(hostDev); err != nil {
				errs = errors.Join(errs, err)
			}
			if err := t.linkDelIn(tmpGuestDev, netns.Path()); err != nil {
				errs = errors.Join(errs, err)
			}
			return errs
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			var errs error
			if err := t.linkDel(hostDev); err != nil {
				errs = errors.Join(errs, err)
			}
			if err := t.linkDelIn(guestDev, netns.Path()); err != nil {
				errs = errors.Join(errs, err)
			}
			return errs
		})
		if err := t.linkSetUpIn(guestDev, netns.Path()); err != nil {
			return err
		}
		if err := t.linkSetMacIn(guestDev, t.MacAddr, netns.Path()); err != nil {
			return err
		}
		if err := t.startOVSPort(ctx, hostDev); err != nil {
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

func (t *T) stopOVS(ctx context.Context) error {
	var hostDev string

	pid, err := t.getNSPID(ctx)
	if err != nil {
		return err
	}
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
	if guestDev != "" {
		hostDev = fmt.Sprintf("v%spl%d", guestDev, pid)
	} else if t.NSDev != "" {
		hostDev = fmt.Sprintf("v%spl%d", t.NSDev, pid)
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
	if err := t.stopOVSPort(hostDev); err != nil {
		return err
	}
	if err := t.stopLink(hostDev); err != nil {
		if errors.Is(err, ErrLinkNotFound) {
			return nil
		}
		return err
	}
	return nil
}
