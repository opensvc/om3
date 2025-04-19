package omcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdObjectStatus struct {
		commoncmd.OptsLock
		ObjectSelector string
		Refresh        bool
		Monitor        bool
	}
)

func (t *CmdObjectStatus) Run(kind string) error {
	p, err := naming.ParsePath(t.ObjectSelector)
	if err != nil {
		return err
	}
	o, err := object.NewCore(p)
	if err != nil {
		return err
	}
	ctx := context.Background()
	ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
	ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
	var instanceStatus instance.Status
	if t.Monitor {
		instanceStatus, err = o.MonitorStatus(ctx)
	} else if t.Refresh {
		instanceStatus, err = o.FreshStatus(ctx)
	} else {
		instanceStatus, err = o.Status(ctx)
	}
	if env.HasDaemonOrigin() {
		return err
	}
	// backward compat
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(int(instanceStatus.Avail))
	return err
}
