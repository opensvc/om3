//go:build linux

package pg

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/systemd"
)

// systemctl sets the unit properties, and is replaced by the tests, which
// make groups of their own but must not make units of the host's systemd.
var systemctl = func(opts ...funcopt.O) *command.T {
	return command.New(opts...)
}

// cpuQuotaPeriod is the period systemd converts a CPUQuota percentage with.
const cpuQuotaPeriod = uint64(100000)

// unit is the systemd slice the group is, and empty for a group that is not
// one.
//
// Every group om makes is named as a nested slice, so that an engine given
// the last element as its cgroup parent asks systemd for the same group.
func (c Config) unit() string {
	name := path.Base(c.ID)
	if !strings.HasSuffix(name, ".slice") {
		return ""
	}
	return name
}

// systemdProperties are the unit properties saying the cappings of the group,
// in the words of systemd.
//
// A group an engine places a container in through systemd is a unit of it,
// and systemd writes the cgroup files of a unit from its properties whenever
// it starts the unit: a capping written in the files alone was put back to
// what the properties said, which is no capping at all, the next time a
// container started in the group. Setting the properties is what makes a
// capping hold.
func (c Config) systemdProperties() ([]string, error) {
	l := make([]string, 0)
	switch c.CPUShares {
	case "":
	case DefaultValue:
		l = append(l, "CPUWeight=")
	default:
		n, err := sizeconv.FromSize(c.CPUShares)
		if err != nil {
			return nil, fmt.Errorf("pg_cpu_shares: %w", err)
		}
		l = append(l, "CPUWeight="+strconv.FormatInt(n, 10))
	}
	switch c.CPUs {
	case "":
	case DefaultValue:
		l = append(l, "AllowedCPUs=")
	default:
		l = append(l, "AllowedCPUs="+c.CPUs)
	}
	switch c.Mems {
	case "":
	case DefaultValue:
		l = append(l, "AllowedMemoryNodes=")
	default:
		l = append(l, "AllowedMemoryNodes="+c.Mems)
	}
	switch c.CPUQuota {
	case "":
	case DefaultValue:
		l = append(l, "CPUQuota=")
	default:
		quota, err := CPUQuota(c.CPUQuota).Convert(cpuQuotaPeriod)
		if err != nil {
			return nil, fmt.Errorf("pg_cpu_quota: %w", err)
		}
		// The quota in microseconds per 100ms period is a percentage of
		// one cpu to a thousandth: systemd takes it to a hundredth.
		l = append(l, "CPUQuota="+strconv.FormatFloat(float64(quota)/1000, 'f', -1, 64)+"%")
	}
	switch c.MemLimit {
	case "":
	case DefaultValue:
		l = append(l, "MemoryMax=infinity")
		if c.VMemLimit == DefaultValue {
			l = append(l, "MemorySwapMax=infinity")
		}
	default:
		n, err := sizeconv.FromSize(c.MemLimit)
		if err != nil {
			return nil, fmt.Errorf("pg_mem_limit: %w", err)
		}
		l = append(l, "MemoryMax="+strconv.FormatInt(n, 10))
		if c.VMemLimit != "" {
			v, err := sizeconv.FromSize(c.VMemLimit)
			if err != nil {
				return nil, fmt.Errorf("pg_vmem_limit: %w", err)
			}
			l = append(l, "MemorySwapMax="+strconv.FormatInt(v-n, 10))
		}
	}
	switch c.BlockIOWeight {
	case "":
	case DefaultValue:
		l = append(l, "IOWeight=")
	default:
		l = append(l, "IOWeight="+c.BlockIOWeight)
	}
	return l, nil
}

// setSystemdProperties gives the cappings of the group to the systemd
// instance managing it: the system's, or the one of the user a delegated
// group belongs to.
//
// They are runtime properties: the groups are made again by the actions
// after a reboot, from the configuration, which is where the cappings
// persist.
func (c Config) setSystemdProperties() error {
	unit := c.unit()
	if unit == "" || !systemd.HasSystemd() {
		return nil
	}
	props, err := c.systemdProperties()
	if err != nil || len(props) == 0 {
		return err
	}
	args := []string{"set-property", "--runtime", unit}
	opts := []funcopt.O{
		command.WithName("systemctl"),
		command.WithBufferedStderr(),
	}
	if d := c.Delegation; d != nil {
		runtimeDir := fmt.Sprintf("/run/user/%d", d.UID)
		args = append([]string{"--user"}, args...)
		opts = append(opts,
			command.WithUser(strconv.Itoa(d.UID)),
			command.WithGroup(strconv.Itoa(d.GID)),
			command.WithVarEnv(
				"XDG_RUNTIME_DIR="+runtimeDir,
				"DBUS_SESSION_BUS_ADDRESS=unix:path="+runtimeDir+"/bus",
			),
			command.WithCWD("/"),
		)
	}
	args = append(args, props...)
	cmd := systemctl(append(opts, command.WithArgs(args))...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg %s: systemctl %s: %w: %s", c.Path(), strings.Join(args, " "), err, strings.TrimSpace(string(cmd.Stderr())))
	}
	return nil
}
