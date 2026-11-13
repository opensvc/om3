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

// ActiveState returns the ActiveState of the opensvc unit: "active",
// "activating", "deactivating", "reloading", "inactive" or "failed".
func (t *T) ActiveState(ctx context.Context) (string, error) {
	prop, err := t.conn.GetUnitPropertyContext(ctx, UnitName, "ActiveState")
	if err != nil {
		return "", err
	}
	if prop == nil {
		return "", nil
	}
	if v, ok := prop.Value.Value().(string); ok {
		return v, nil
	}
	return strings.Trim(prop.Value.String(), "\""), nil
}

// CalledFromManager detects if current process as been launched by systemd:
// it is the main process of the unit, the "om daemon run" of its ExecStart,
// or its control process, the "om daemon stop" of its ExecStop.
//
// A process of the unit cgroup, or with its INVOCATION_ID, is not: every
// child of the daemon is there, as the "om cluster join" it forks, which,
// taken for systemd, stopped the daemon through the api and started another
// itself, out of the unit. The pids of the unit are said by systemd 219 too.
func (t *T) CalledFromManager() bool {
	if t.conn == nil {
		return false
	}
	pids := make(map[string]uint32)
	for _, name := range unitPIDProperties {
		prop, err := t.conn.GetUnitTypePropertyContext(context.Background(), UnitName, "Service", name)
		if err != nil || prop == nil {
			continue
		}
		if v, ok := prop.Value.Value().(uint32); ok {
			pids[name] = v
		}
	}
	return isUnitPID(uint32(os.Getpid()), pids)
}

// unitPIDProperties are the properties of a service unit holding the pids
// of the processes systemd runs for it.
var unitPIDProperties = []string{"MainPID", "ControlPID"}

// isUnitPID says pid is one of the pids of a unit, 0 for none.
func isUnitPID(pid uint32, pids map[string]uint32) bool {
	for _, v := range pids {
		if v != 0 && v == pid {
			return true
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

// Start starts the opensvc systemd unit, and returns when systemd ended the
// job: an error when it is not done.
func (t *T) Start(ctx context.Context) error {
	c := make(chan string, 1)
	if _, err := t.conn.StartUnitContext(ctx, UnitName, "replace", c); err != nil {
		return err
	}
	return waitJob(ctx, "start", c)
}

// Stop stops the opensvc systemd unit, and returns when systemd ended the
// job: an error when it is not done. The "replace" mode makes the stop
// replace a start or a restart queued or running: the stop wins.
func (t *T) Stop(ctx context.Context) error {
	c := make(chan string, 1)
	if _, err := t.conn.StopUnitContext(ctx, UnitName, "replace", c); err != nil {
		return err
	}
	return waitJob(ctx, "stop", c)
}

// waitJob waits for the result systemd gives the job of op, and returns an
// error when it is not "done": "canceled" when another job replaced it, as a
// stop replaces a restart, "failed", "timeout", "dependency" or "skipped".
//
// The channel must be buffered: go-systemd sends the result holding the lock
// of its job listener, so a send nobody reads would block the result of every
// later job of the connection.
func waitJob(ctx context.Context, op string, c <-chan string) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("%s %s: %w", op, UnitName, ctx.Err())
	case result := <-c:
		return jobResultError(op, result)
	}
}

// jobResultError returns an error for a job result other than "done".
func jobResultError(op, result string) error {
	if result == "done" {
		return nil
	}
	return fmt.Errorf("%s %s: job %s", op, UnitName, result)
}

// New provides a connected object to dbus systemd that implement following interfaces:
//
//	ActiveState(ctx context.Context) (string, error)
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
