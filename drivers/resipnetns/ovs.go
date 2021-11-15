package resipnetns

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/util/command"
)

func (t *T) startOVSPort(ctx context.Context, dev string) error {
	args := []string{
		"--may-exist",
		"add-port", t.IpDev, dev,
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

	actionrollback.Register(ctx, func() error {
		return t.stopOVSPort(dev)
	})
	return nil
}

func (t *T) stopOVSPort(dev string) error {
	if dev == "" {
		return nil
	}
	args := []string{
		"--if-exist",
		"del-port", t.IpDev, dev,
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
	return nil
}

func (t *T) startOVS(ctx context.Context) error {
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
	hostDev := formatHostDevName(guestDev, pid)

	mtu, err := t.devMTU()
	if err != nil {
		return err
	}

	if err := t.startVEthPair(ctx, netns, hostDev, guestDev, mtu); err != nil {
		return err
	}
	if err := t.startOVSPort(ctx, hostDev); err != nil {
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

func (t *T) stopOVS(ctx context.Context) error {
	var hostDev string

	pid, err := t.getNSPID()
	if err != nil {
		return err
	}
	netns, err := t.getNS()
	if err != nil {
		return err
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
		return err
	}
	if err := t.stopOVSPort(hostDev); err != nil {
		return err
	}
	if err := t.stopVEthPair(hostDev); err != nil {
		return err
	}
	return nil
}
