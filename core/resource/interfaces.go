package resource

import (
	"context"
	"io"

	"github.com/opensvc/om3/core/actionresdeps"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/util/device"
)

type (
	//
	// Aborter implements the Abort func, which can return true to
	// block a start action before any resource has entered Start().
	//
	Aborter interface {
		Abort(ctx context.Context) bool
	}

	//
	// ActionResourceDepser implements the ActionResourceDeps func, which
	// return a list of {action, rid depending on, rid depended on} tuples.
	//
	ActionResourceDepser interface {
		ActionResourceDeps() []actionresdeps.Dep
	}

	// Configurer is an optional interface resource drivers can
	// implement if they want to configure the resource after the
	// manifest data has been loaded. For example, tuning the logger.
	Configurer interface {
		Configure() error
	}

	SetSSHKeyFiler interface {
		SetSSHKeyFile()
	}

	// PreMove implements a PreMove function that is called by a moveable
	// driver (eg container.kvm) before starting a move.
	PreMover interface {
		PreMove(ctx context.Context, to string) error
	}

	//
	// Runner implements the Run func, which runs a one-shot process
	// Implemented by the resource. The object "run" action causes
	// selected Runners to call Run().
	//
	Runner interface {
		Run(ctx context.Context) error
	}

	//
	// Runninger implements the Running func, which the core calls
	// when evaluating an object instance status to build the "running"
	// list: [{"pid": 123, "rid": "task#1", "session_id": "abcd..."}]
	//
	Runninger interface {
		Running() (RunningInfoList, error)
	}

	//
	// Scheduler implements the Schedules func, which returns the list of
	// schedulable job definition on behalf of the resource.
	//
	Scheduler interface {
		Schedules() schedule.Table
	}

	//
	// StatusInfoer implements the StatusInfo func, which returns a
	// resource specific key-val mapping pushed to the collector on
	// "pushinfo" action.
	//
	StatusInfoer interface {
		StatusInfo(context.Context) map[string]interface{}
	}

	// NetNSPather exposes a NetNSPath method a resource can call to
	// get the string identifying the network namespace for libs like
	// netlink.
	// For example, the container.docker driver's NetNSPath() would return
	// the SandboxKey
	NetNSPather interface {
		NetNSPath(context.Context) (string, error)
	}

	// PIDer exposes a PID method a resource can call to
	// get the head pid of the head process started by the resource.
	// Typically a container resource PID() returns the pid of the
	// first process of the container.
	// PID() must return 0 when no process is running.
	PIDer interface {
		PID(context.Context) int
	}

	// GetHostnamer exposes a GetHostname method a resource can call
	// to get the hostname used by ip resources to obtain a
	// hostname-based DNS record
	GetHostnamer interface {
		GetHostname() string
	}

	shutdowner interface {
		Shutdown(context.Context) error
	}
	starter interface {
		Start(context.Context) error
	}
	startstandbyer interface {
		StartStandby(context.Context) error
	}
	stopstandbyer interface {
		StopStandby(context.Context) error
	}
	stopper interface {
		Stop(context.Context) error
	}
	booter interface {
		Boot(ctx context.Context) error
	}
	resyncer interface {
		Resync(context.Context) error
	}
	fuller interface {
		Full(context.Context) error
	}
	updater interface {
		Update(context.Context) error
	}
	ingester interface {
		Ingest(context.Context) error
	}
	SubDeviceser interface {
		SubDevices() device.L
	}

	Commander interface {
		CombinedOutput() ([]byte, error)
		Run() error
		Start() error
		StderrPipe() (io.ReadCloser, error)
		Wait() error
	}
	Encaper interface {
		GetHostname() string
		GetOsvcRootPath() string
		EncapCp(context.Context, string, string) error
		EncapCmd(ctx context.Context, args []string, envs []string, stdin io.Reader) (Commander, error)
	}
)
