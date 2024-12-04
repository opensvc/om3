package resipnetns

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/util/command"
	"github.com/rs/zerolog"
)

func (t *T) sysctlEnableIPV6In(dev, path string) error {
	cmd := command.New(
		command.WithName("nsenter"),
		command.WithArgs([]string{"--net=" + path, "sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6=0", dev)}),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
	)
	return cmd.Run()
}

func (t *T) sysctlDisableIPV6RA(dev string) error {
	cmd := command.New(
		command.WithName("sysctl"),
		command.WithArgs([]string{"-w", fmt.Sprintf("net.ipv6.conf.%s.accept_ra=0", dev)}),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.WarnLevel),
	)
	return cmd.Run()
}

func (t *T) linkListIn(path string) (string, error) {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "list"}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
	)
	err := cmd.Run()
	return string(cmd.Stdout()), err
}

func (t *T) linkDel(dev string) error {
	isNotFound := false
	args := []string{"ip", "link", "del", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithOnStderrLine(func(line string) {
			if strings.Contains(line, "Cannot find device") {
				isNotFound = true
			} else {
				t.Log().Errorf("stderr: " + line)
			}
		}),
	)
	err := cmd.Run()
	if isNotFound {
		return nil
	}
	return err
}

func (t *T) linkDelIn(dev, path string) error {
	isNotFound := false
	args := []string{"nsenter", "--net=" + path, "ip", "link", "del", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithOnStderrLine(func(line string) {
			if strings.Contains(line, "Cannot find device") {
				isNotFound = true
			} else {
				t.Log().Errorf("stderr: " + line)
			}
		}),
	)
	err := cmd.Run()
	if isNotFound {
		return nil
	}
	return err
}

func (t *T) linkSetMaster(dev, master string) error {
	args := []string{"ip", "link", "set", dev, "master", master}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) linkSetNsPid(dev string, pid int) error {
	args := []string{"ip", "link", "set", dev, "netns", fmt.Sprint(pid)}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) linkSetNameIn(dev, name, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "set", dev, "name", name}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) linkSetUp(dev string) error {
	args := []string{"ip", "link", "set", dev, "up"}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) linkSetUpIn(dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "set", dev, "up"}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) linkSetMacIn(dev, address, path string) error {
	if address == "" {
		return nil
	}
	args := []string{"nsenter", "--net=" + path, "ip", "link", "set", dev, "address", address}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) makeIPVLANDev(dev1, dev2 string, mtu int, mode string) error {
	mtuS := fmt.Sprint(mtu)
	args := []string{"ip", "link", "add", "link", dev1, "dev", dev2, "mtu", mtuS, "type", "ipvlan", "mode", mode}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) makeMACVLANDev(dev1, dev2 string, mtu int) error {
	mtuS := fmt.Sprint(mtu)
	args := []string{"ip", "link", "add", "link", dev1, "dev", dev2, "mtu", mtuS, "type", "macvlan", "mode", "bridge"}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) makeVethPair(dev1, dev2 string, mtu int) error {
	mtuS := fmt.Sprint(mtu)
	args := []string{"ip", "link", "add", "name", dev1, "mtu", mtuS, "type", "veth", "peer", "name", dev2, "mtu", mtuS}

	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) addrAddIn(addr, dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "addr", "add", addr, "dev", dev}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) addrDelIn(addr, dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "addr", "del", addr, "dev", dev}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) addrListIn(dev, path string) (string, error) {
	args := []string{"nsenter", "--net=" + path, "ip", "addr", "list", "dev", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
	)
	err := cmd.Run()
	return string(cmd.Stdout()), err
}

func (t *T) hasLink(dev string) (bool, error) {
	args := []string{"ip", "link", "list", "dev", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithIgnoredExitCodes(0, 1),
	)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if len(cmd.Stdout()) != 0 {
		return true, nil
	}
	return false, nil
}

func (t *T) hasLinkIn(dev, path string) (bool, error) {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "list", "dev", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithIgnoredExitCodes(0, 1),
	)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if len(cmd.Stdout()) != 0 {
		return true, nil
	}
	return false, nil
}

func (t *T) hasRouteDevIn(dest, dev, path string) (bool, error) {
	args := []string{"nsenter", "--net=" + path, "ip", "route", "list", dest, "dev", dev}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if len(cmd.Stdout()) != 0 {
		return true, nil
	}
	return false, nil
}

func (t *T) hasRouteViaIn(dest, gw, path string) (bool, error) {
	args := []string{"nsenter", "--net=" + path, "ip", "route", "list", dest, "gw", gw}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if len(cmd.Stdout()) != 0 {
		return true, nil
	}
	return false, nil
}

func (t *T) routeDelDevIn(dest, dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "route", "del", dest, "dev", dev}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) routeAddDevIn(dest, dev, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "route", "replace", dest, "dev", dev}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}

func (t *T) routeAddViaIn(dest, gw, path string) error {
	args := []string{"nsenter", "--net=" + path, "ip", "route", "replace", dest, "via", gw}
	return command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	).Run()
}
