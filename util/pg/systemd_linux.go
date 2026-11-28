//go:build linux

package pg

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"

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

// hasSystemd says whether the node runs systemd, and is replaced by the tests.
var hasSystemd = systemd.HasSystemd

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
	switch c.MemHigh {
	case "":
	case DefaultValue:
		l = append(l, "MemoryHigh=infinity")
	default:
		n, err := sizeconv.FromSize(c.MemHigh)
		if err != nil {
			return nil, fmt.Errorf("pg_mem_high: %w", err)
		}
		l = append(l, "MemoryHigh="+strconv.FormatInt(n, 10))
	}
	switch c.PidsMax {
	case "":
	case DefaultValue:
		l = append(l, "TasksMax=infinity")
	default:
		l = append(l, "TasksMax="+c.PidsMax)
	}
	// pg_cpu_burst has no property: systemd does not manage cpu.max.burst,
	// so it neither sets it nor puts it back. The file is written at every
	// apply, which is before each container starts in the group.
	switch c.BlockIOWeight {
	case "":
	case DefaultValue:
		l = append(l, "IOWeight=")
	default:
		l = append(l, "IOWeight="+c.BlockIOWeight)
	}
	return l, nil
}

// v1Property is a capping of the group as the systemd of a node of the v1
// hierarchy holds it: a property of the slice, of the D-Bus type t.
type v1Property struct {
	name string

	// controller is the v1 hierarchy systemd writes the property in, as
	// it is mounted under the cgroup mount.
	controller string

	// dropIn is the file a systemd older than 236 keeps the property in,
	// under the drop-in directory of the slice, see forgetV1Properties.
	dropIn string

	value uint64
}

// v1PropertyUnset is the value of a v1Property nothing caps: the infinity
// of MemoryLimit, TasksMax and CPUQuotaPerSecUSec, and the weight not set of
// CPUShares and BlockIOWeight, which leaves the kernel default.
const v1PropertyUnset = uint64(math.MaxUint64)

// The shares systemd takes are the ones the kernel keeps: it holds a
// cpu.shares written out of them to the nearest of the two.
const (
	v1CPUSharesMin = uint64(2)
	v1CPUSharesMax = uint64(262144)
)

// v1PropertyKinds are the properties systemdV1Properties may return, in
// the order it returns them.
var v1PropertyKinds = []v1Property{
	{name: "CPUShares", controller: "cpu", dropIn: "50-CPUShares.conf"},
	{name: "CPUQuotaPerSecUSec", controller: "cpu", dropIn: "50-CPUQuota.conf"},
	{name: "MemoryLimit", controller: "memory", dropIn: "50-MemoryLimit.conf"},
	{name: "TasksMax", controller: "pids", dropIn: "50-TasksMax.conf"},
	{name: "BlockIOWeight", controller: "blkio", dropIn: "50-BlockIOWeight.conf"},
}

// systemdV1Properties are the unit properties saying the cappings of the
// group on a node of the v1 or the hybrid hierarchy, in the words the
// systemd releases of these nodes know, from 219 on: MemoryLimit, CPUShares
// and BlockIOWeight, where the unified hierarchy has MemoryMax, CPUWeight
// and IOWeight.
//
// A slice is a unit there too, and one a container is started in becomes
// an active one: systemd then writes the files of its group from its
// properties, and again at every daemon-reload. A capping written in the
// files alone was lifted as the container started.
//
// The quota is CPUQuotaPerSecUSec, the property systemctl makes of a
// CPUQuota percentage: the cpu time per second, which systemd writes for
// its own period of 100ms. That period is the one om writes, so there is no
// CPUQuotaPeriodUSec to give, which a systemd older than 242 does not know.
//
// The other cappings have no property on these nodes, and stay in the
// files alone, where systemd leaves them as they are, managing none of
// them: pg_cpus and pg_mems, the cpuset controller being none of systemd's
// on the v1 hierarchy, pg_vmem_limit, memory.memsw.limit_in_bytes,
// pg_mem_swappiness and pg_mem_oom_control.
func (c Config) systemdV1Properties() ([]v1Property, error) {
	l := make([]v1Property, 0)
	set := func(name string, value uint64) {
		for _, p := range v1PropertyKinds {
			if p.name == name {
				p.value = value
				l = append(l, p)
			}
		}
	}
	switch c.CPUShares {
	case "":
	case DefaultValue:
		set("CPUShares", v1PropertyUnset)
	default:
		n, err := sizeconv.FromSize(c.CPUShares)
		if err != nil {
			return nil, fmt.Errorf("pg_cpu_shares: %w", err)
		}
		set("CPUShares", min(max(uint64(n), v1CPUSharesMin), v1CPUSharesMax))
	}
	switch c.CPUQuota {
	case "":
	case DefaultValue:
		set("CPUQuotaPerSecUSec", v1PropertyUnset)
	default:
		quota, err := CPUQuota(c.CPUQuota).Convert(cpuQuotaPeriod)
		if err != nil {
			return nil, fmt.Errorf("pg_cpu_quota: %w", err)
		}
		set("CPUQuotaPerSecUSec", uint64(quota)*(1000000/cpuQuotaPeriod))
	}
	switch c.MemLimit {
	case "":
	case DefaultValue:
		set("MemoryLimit", v1PropertyUnset)
	default:
		n, err := sizeconv.FromSize(c.MemLimit)
		if err != nil {
			return nil, fmt.Errorf("pg_mem_limit: %w", err)
		}
		set("MemoryLimit", uint64(n))
	}
	switch c.PidsMax {
	case "":
	case DefaultValue:
		set("TasksMax", v1PropertyUnset)
	default:
		n, err := strconv.ParseUint(c.PidsMax, 10, 64)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("pg_pids_max: %s is not a count of processes", c.PidsMax)
		}
		set("TasksMax", n)
	}
	switch c.BlockIOWeight {
	case "":
	case DefaultValue:
		set("BlockIOWeight", v1PropertyUnset)
	default:
		n, err := strconv.ParseUint(c.BlockIOWeight, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("pg_blkio_weight: %w", err)
		}
		set("BlockIOWeight", n)
	}
	return l, nil
}

// v1Mounted keeps the properties of the controllers the node mounts: one it
// does not mount caps nothing, and its property may be one the systemd of
// the node does not know, as TasksMax where there is no pids controller. The
// writes of applyV1Resets skip the same.
func v1Mounted(props []v1Property) []v1Property {
	l := make([]v1Property, 0, len(props))
	for _, p := range props {
		if _, err := os.Stat(filepath.Join(cgroupMount, p.controller)); errors.Is(err, os.ErrNotExist) {
			continue
		}
		l = append(l, p)
	}
	return l
}

// errNoSuchUnit is the name of the error systemd answers with for a unit it
// has not loaded.
const errNoSuchUnit = "org.freedesktop.systemd1.NoSuchUnit"

// isNoSuchUnit says err is systemd refusing a unit it has not loaded.
func isNoSuchUnit(err error) bool {
	var e dbus.Error
	return errors.As(err, &e) && e.Name == errNoSuchUnit
}

// startUnit and stopUnit queue the start and the stop of a unit of the
// systemd of the system, and are replaced by the tests.
//
// The job of a start is not waited for: systemd has the unit loaded when it
// answers, which is what a start is asked for here.
//
// The job of a stop is waited for when wait is set, see stopUnitJob: a
// start of the object following its stop would otherwise give systemd the
// properties of a slice whose stop is still queued.
var (
	startUnit = func(ctx context.Context, unit string) error {
		conn, err := systemdConn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = conn.StartUnitContext(ctx, unit, "replace", nil)
		return err
	}
	stopUnit = func(ctx context.Context, unit string, wait bool) error {
		conn, err := systemdConn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		if !wait {
			_, err = conn.StopUnitContext(ctx, unit, "replace", nil)
			return err
		}
		// systemd signals the end of a job to every connection of its
		// own socket, and to the system bus when a client of it
		// subscribed only. A subscription refused leaves the state of
		// the unit to tell the end.
		_ = conn.Subscribe()
		job := func(ctx context.Context, done chan<- string) error {
			_, err := conn.StopUnitContext(ctx, unit, "replace", done)
			return err
		}
		state := func(ctx context.Context) (string, error) {
			prop, err := conn.GetUnitPropertyContext(ctx, unit, "ActiveState")
			if err != nil {
				return "", err
			}
			s, _ := prop.Value.Value().(string)
			return s, nil
		}
		return stopUnitJob(ctx, unit, job, state)
	}
)

// unitStopPoll is the time between two reads of the state of a unit whose
// stop is waited for. It is replaced by the tests.
var unitStopPoll = 200 * time.Millisecond

// errUnitStopPending is the error of a stop systemd queued, and that was not
// seen done in the time given: the unit may still be active.
var errUnitStopPending = errors.New("the stop job is still pending")

// stopUnitJob queues the stop of unit with job, and returns when the stop is
// done, or with errUnitStopPending when ctx ends first.
//
// systemd answers a stop when its job is queued, not when it has run. On
// the v1 hierarchy, a start of the object coming right after, as the one of
// a restart, could reach systemd first: it gave their properties to slices
// still loaded, then the stop ran, the slices went inactive and systemd
// removed their groups, still empty, with the caps it holds no property
// for, as pg_vmem_limit, pg_cpus and pg_mems, lost until the next start.
//
// The stop is done when the job says its result on done, whatever it is: a
// job canceled or failed is no longer queued either. It is done too when
// state, read every unitStopPoll, says the unit inactive, failed or not
// loaded: systemd runs the job and removes the groups of the unit in the
// step it changes its state in, and the signal of a job is not something
// every systemd and every connection to it are known to deliver, the 219
// of rhel7 on the system bus among them.
func stopUnitJob(ctx context.Context, unit string, job func(context.Context, chan<- string) error, state func(context.Context) (string, error)) error {
	// The result is sent under the lock of the connection: a full
	// channel would block every job of it.
	done := make(chan string, 1)
	if err := job(ctx, done); err != nil {
		return err
	}
	ticker := time.NewTicker(unitStopPoll)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return nil
		case <-ticker.C:
			s, err := state(ctx)
			if isNoSuchUnit(err) || (err == nil && (s == "inactive" || s == "failed")) {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("%s: %w: %w", unit, errUnitStopPending, ctx.Err())
		}
	}
}

// setUnitV1Properties gives props to the systemd of the system, as runtime
// properties of unit.
//
// They are given over D-Bus, as the network filter is, not as arguments of
// systemctl, which gives the same ones: a value is then the number systemd
// holds, and a lifted capping the one value every release takes for it.
// The systemctl of systemd 219 has no word for a MemoryLimit lifted, taking
// neither infinity nor an empty assignment.
//
// An old systemd, as the 219 of rhel7, sets the properties of the units it
// has loaded only, and has loaded no slice nothing was started
// in: it refuses the one of a group om makes. With load set, the
// slice is then started, which loads it and is what the engine starting a
// container in it does anyway, and given its properties again. The slice
// is stopped with its group, see RemoveTree. Without load, a unit not
// loaded is left so, and is no error: it holds nothing to unset.
//
// Started, and given properties, the slice is realized: systemd removes its
// group from the hierarchies its properties do not need, when it holds
// nothing there, so the caller makes the group after, see ApplyProc.
func (c Config) setUnitV1Properties(unit string, props []v1Property, load bool) error {
	if len(props) == 0 {
		return nil
	}
	names := make([]string, 0, len(props))
	l := make([]sddbus.Property, 0, len(props))
	for _, p := range props {
		names = append(names, p.name)
		l = append(l, sddbus.Property{Name: p.name, Value: dbus.MakeVariant(p.value)})
	}
	ctx, cancel := context.WithTimeout(context.Background(), ipFilterTimeout)
	defer cancel()
	err := setUnitProperties(ctx, unit, l...)
	if isNoSuchUnit(err) {
		if !load {
			return nil
		}
		if err := startUnit(ctx, unit); err != nil {
			return fmt.Errorf("start %s, for systemd to take its %s: %w", unit, strings.Join(names, ", "), err)
		}
		if c.log != nil {
			c.log.Infof("started the systemd slice %s, for systemd to take its properties", unit)
		}
		err = setUnitProperties(ctx, unit, l...)
	}
	if err != nil {
		return fmt.Errorf("set the %s of %s: %w", strings.Join(names, ", "), unit, err)
	}
	if c.log != nil {
		c.log.Debugf("set the systemd properties %s of %s", strings.Join(names, ", "), unit)
	}
	return nil
}

// setSystemdProperties gives the cappings of the group to the systemd
// instance managing it: the system's, or the one of the user a delegated
// group belongs to.
//
// They are runtime properties: the groups are made again by the actions
// after a reboot, from the configuration, which is where the cappings
// persist.
//
// The properties are the ones of the hierarchy of the node: a group of the
// v1 hierarchy is given systemdV1Properties, and is never a delegated one.
func (c Config) setSystemdProperties() error {
	unit := c.unit()
	if unit == "" || !hasSystemd() {
		return nil
	}
	if !isUnified() {
		props, err := c.systemdV1Properties()
		if err != nil {
			return err
		}
		return c.setUnitV1Properties(unit, v1Mounted(props), true)
	}
	props, err := c.systemdProperties()
	if err != nil || len(props) == 0 {
		return err
	}
	args := append([]string{"set-property", "--runtime", unit}, props...)
	return runSystemctl(c.Delegation, args)
}

// runSystemctl runs systemctl with args, against the systemd instance of the
// user a delegated group belongs to, or the one of the system when d is nil.
func runSystemctl(d *Delegation, args []string) error {
	opts := []funcopt.O{
		command.WithName("systemctl"),
		command.WithBufferedStderr(),
	}
	if d != nil {
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
	cmd := systemctl(append(opts, command.WithArgs(args))...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(cmd.Stderr())))
	}
	return nil
}
