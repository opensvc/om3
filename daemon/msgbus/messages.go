package msgbus

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	// Msg wraps any other type so the chan can accept Msg instead of the unrestricted "any"
	Msg any

	Exit struct {
		Path     path.T
		Filename string
	}

	// CfgFileUpdated is emited by a fs watcher when a .conf file is updated or created in etc.
	// The smon goroutine listens to this event and updates the daemondata, which in turns emits a CfgUpdated{} event.
	CfgFileUpdated struct {
		Path     path.T
		Filename string
	}

	// CfgFileRemoved is emited by a fs watcher when a .conf file is removed in etc.
	// The smon goroutine listens to this event and updates the daemondata, which in turns emits a CfgDeleted{} event.
	CfgFileRemoved struct {
		Path     path.T
		Filename string
	}

	// FrozenFileUpdated is emited by a fs watcher when a frozen file is updated or created in var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a Frozen{} event.
	FrozenFileUpdated struct {
		Path     path.T
		Filename string
	}

	// FrozenFileRemoved is emited by a fs watcher when a frozen file is removed from var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a Frozen{} event.
	FrozenFileRemoved struct {
		Path     path.T
		Filename string
	}

	CfgDeleted struct {
		Path path.T
		Node string
	}

	CfgUpdated struct {
		Path   path.T
		Node   string
		Config instance.Config
	}

	MonCfgDone struct {
		Path     path.T
		Filename string
	}

	RemoteFileConfig struct {
		Path     path.T
		Node     string
		Filename string
		Updated  time.Time
		Ctx      context.Context
		Err      chan error
	}

	Frozen struct {
		Path  path.T
		Node  string
		Value time.Time
	}

	InstStatusDeleted struct {
		Path path.T
		Node string
	}

	InstStatusUpdated struct {
		Path   path.T
		Node   string
		Status instance.Status
	}

	SetNmon struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	NmonDeleted struct {
		Node string
	}

	NmonUpdated struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	SetSmon struct {
		Path    path.T
		Node    string
		Monitor instance.Monitor
	}

	SmonDeleted struct {
		Path path.T
		Node string
	}

	SmonUpdated struct {
		Path   path.T
		Node   string
		Status instance.Monitor
	}

	MonSvcAggDeleted struct {
		Path path.T
		Node string
	}

	MonSvcAggUpdated struct {
		Path   path.T
		Node   string
		SvcAgg object.AggregatedStatus
		SrcEv  *Msg
	}

	MonSvcAggDone struct {
		Path path.T
	}
)

func NewMsg(arg interface{}) *Msg {
	var t Msg
	t = arg
	return &t
}

func DropPendingMsg(c <-chan *Msg, duration time.Duration) {
	dropping := make(chan bool)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		dropping <- true
		for {
			select {
			case <-c:
			case <-ctx.Done():
				return
			}
		}
	}()
	<-dropping
}
