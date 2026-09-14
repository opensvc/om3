package resource

import (
	"context"
	"io"

	"github.com/opensvc/om3/v3/core/actionresdeps"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/util/device"
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

	// Mover implements a Move function that is exposed by resource
	// drivers that support live migration (eg container.kvm).
	Mover interface {
		Move(ctx context.Context, to string) error
	}

	// PreMover implements a PreMove function that is called by a moveable
	// driver (eg container.kvm) before starting a move.
	PreMover interface {
		PreMove(ctx context.Context, to string) error
	}

	// PreMoveRollbacker implements a PreMove rollback function that is called
	// by a moveable driver (eg container.kvm) when move has failed.
	PreMoveRollbacker interface {
		PreMoveRollback(ctx context.Context, to string) error
	}

	// PostMover implements a PostMove function that is called by a moveable
	// driver (eg container.kvm) after the move is done.
	PostMover interface {
		PostMove(ctx context.Context, to string) error
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
	splitter interface {
		Split(context.Context) error
	}
	fuller interface {
		Full(context.Context) error
	}
	updater interface {
		Update(context.Context) error
	}
	datastoreLister interface {
		DatastoreList(context.Context) naming.Paths
	}
	toSyncer interface {
		ToSync(context.Context) []string
	}
	ingester interface {
		Ingest(context.Context) error
	}
	SubDeviceser interface {
		SubDevices(context.Context) device.L
	}

	// Sizer is implemented by a resource driver that knows how much space it
	// holds. It is what a relative resize resolves against, and what decides
	// whether a resize grows or shrinks.
	//
	// It is not called Size because a driver whose size is configurable holds
	// that keyword in a Size field, and the two are not the same thing: one
	// is what was asked for, this is what is there.
	Sizer interface {
		CurrentSize(ctx context.Context) (int64, error)
	}

	// Resizer is implemented by a resource driver that can change its size.
	//
	// ResizePlan is asked first, of every link of the chain, and changes
	// nothing. It answers the size this resource needs from the resources
	// below it, which is not always the size it was asked for: a raid6 md
	// holding n devices needs to(n-2) from each of them. It errors to refuse,
	// naming what it cannot do, so that a chain is refused whole rather than
	// left half resized - an xfs filesystem cannot shrink at all, and finding
	// that out after the device under it has shrunk is data loss.
	//
	// Resize is asked second, of the same links, once every one of them has
	// agreed.
	Resizer interface {
		ResizePlan(ctx context.Context, to int64) (needBelow int64, err error)
		Resize(ctx context.Context, to int64) error
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
