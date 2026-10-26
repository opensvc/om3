//go:build linux

package daemonsys

import (
	"context"
	"fmt"
	"os"
	"strings"

	sddaemon "github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/opensvc/om3/v3/util/command"
)

type (
	// T handle dbus.Conn for systemd
	T struct {
		conn *dbus.Conn
	}
)

const (
	UnitName = "opensvc-server.service"
)

// Activated detects if opensvc unit is activated
func (t *T) Activated(ctx context.Context) (bool, error) {
	prop, err := t.conn.GetUnitPropertyContext(ctx, UnitName, "ActiveState")
	if err != nil {
		return false, err
	}
	if prop == nil {
		return false, nil
	}
	return prop.Value.String() == "\"active\"", nil
}

// CalledFromManager detects if current process as been launched by systemd.
//
// systemd sets INVOCATION_ID in the environment of the processes of a unit
// since its version 232. Before, as the 219 of RHEL 7, the process is known
// launched by systemd from its cgroup: systemd runs every process of a unit,
// the ExecStop ones too, in the cgroup of the unit.
func (t *T) CalledFromManager() bool {
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false
	}
	return inUnitCgroup(b, UnitName)
}

// inUnitCgroup says if a /proc/<pid>/cgroup content places the process in
// the cgroup of the unit, or one under it: the line of the systemd hierarchy
// on cgroup v1, "name=systemd", or the line of the unified hierarchy, "0::".
func inUnitCgroup(b []byte, unit string) bool {
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			continue
		}
		if fields[1] != "name=systemd" && !(fields[0] == "0" && fields[1] == "") {
			continue
		}
		for _, elem := range strings.Split(fields[2], "/") {
			if elem == unit {
				return true
			}
		}
	}
	return false
}

// Close closes systemd dbus connection
func (t *T) Close() error {
	if t.conn != nil {
		t.conn.Close()
	}
	return nil
}

// Defined verify if opensvc systemd unit exists and returns true and job type
// when unit is loaded
func (t *T) Defined(ctx context.Context) (loaded bool, err error) {
	units, err := t.conn.ListUnitsContext(ctx)
	if err != nil {
		return false, err
	}
	for _, v := range units {
		if v.Name == UnitName && v.LoadState == "loaded" {
			return true, nil
		}
	}
	return false, nil
}

// NotifyWatchdog sends watch dog notify to systemd
func (t *T) NotifyWatchdog() (bool, error) {
	if t.conn == nil {
		return false, nil
	}
	return sddaemon.SdNotify(false, sddaemon.SdNotifyWatchdog)
}

// NotifyReady sends NotifyReady to systemd
func (t *T) NotifyReady() (bool, error) {
	if t.conn == nil {
		return false, nil
	}
	return sddaemon.SdNotify(false, sddaemon.SdNotifyReady)
}

// NotifyStopping sends NotifyStopping to systemd
func (t *T) NotifyStopping() (bool, error) {
	if t.conn == nil {
		return false, nil
	}
	return sddaemon.SdNotify(false, sddaemon.SdNotifyStopping)
}

// Restart restarts the opensvc systemd unit
//
// restart calls systemd-run systemctl restart opensvc-agent. This allows
// the command to be attached on another control group and prevent systemd
// warnings during 'om daemon restart' such as:
//
//	systemd[1]: Stopping OpenSVC server...
//	systemd[1]: opensvc-server.service: Succeeded.
//	systemd[1]: Stopped OpenSVC server.
//	systemd[1]: opensvc-server.service: Found left-over process 2899690 (om) in control group while starting unit. Ignoring.
//	systemd[1]: This usually indicates unclean termination of a previous run, or service implementation deficiencies.
//	systemd[1]: opensvc-server.service: Found left-over process 2899697 (systemctl) in control group while starting unit. Ignoring.
//	systemd[1]: This usually indicates unclean termination of a previous run, or service implementation deficiencies.
//	systemd[1]: Starting OpenSVC server...
//	systemd[1]: Started OpenSVC server.
func (t *T) Restart() error {
	return command.New(
		command.WithName("systemd-run"),
		command.WithVarArgs("systemctl", "restart", UnitName),
	).Run()
}

// Start starts the opensvc systemd unit
func (t *T) Start(ctx context.Context) error {
	c := make(chan string)
	_, err := t.conn.StartUnitContext(ctx, UnitName, "replace", c)
	if err != nil {
		return err
	}
	<-c
	return nil
}

// Stop stops the opensvc systemd unit
func (t *T) Stop(ctx context.Context) error {
	c := make(chan string)
	_, err := t.conn.StopUnitContext(ctx, UnitName, "replace", c)
	if err != nil {
		return err
	}
	<-c
	return nil
}

// New provides a connected object to dbus systemd that implement following interfaces:
//
//	Activated(ctx context.Context) (bool, error)
//	CalledFromManager() bool
//	Close() error
//	NotifyWatchdog() (bool, error)
//	Start(ctx context.Context) error
//	Stop(ctx context.Context) error
func New(ctx context.Context) (*T, error) {
	c, err := dbus.NewSystemdConnectionContext(ctx)
	if err != nil {
		return nil, err
	}
	return &T{conn: c}, nil
}

func (t *T) IsSystemStopping() (bool, error) {
	state, err := t.conn.GetManagerProperty("SystemState")
	if err != nil {
		return false, fmt.Errorf("failed to get system state: %w", err)
	}
	return strings.Trim(state, "\"") == "stopping", nil
}
