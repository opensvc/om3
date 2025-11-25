// Package istat implements the management of local instance status
package istat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	// T is used to publish local instance status updates
	//
	// It publishes following messages for localhost instances:
	//   - msgbus.InstanceStatusDeleted
	//   - msgbus.InstanceStatusUpdated
	T struct {
		localhost string

		// iStatusM is the localhost instances indexed by path
		//
		//   The iStatusM map is updated from:
		//      * local msgbus.InstanceConfigDeleted (delete)
		//      * local msgbus.InstanceStatusPost (set value)
		//      * local msgbus.InstanceFrozenFileUpdated (update value)
		//      * local msgbus.InstanceFrozenFileUpdated (update value)
		//
		//   The value for localhost is the source of localhost publication of
		//    msgbus.InstanceStatusUpdated.
		iStatusM map[string]instance.Status

		log *plog.Logger

		ctx    context.Context
		cancel context.CancelFunc

		publisher pubsub.Publisher

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		labelLocalhost pubsub.Label
		wg             sync.WaitGroup
	}
)

func New(subQS pubsub.QueueSizer) *T {
	localhost := hostname.Hostname()
	return &T{
		iStatusM:       make(map[string]instance.Status),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
	}
}

func (t *T) Start(ctx context.Context) error {
	t.log = plog.NewDefaultLogger().WithPrefix("daemon: istat: ").Attr("pkg", "daemon/istat")
	t.log.Tracef("starting")
	defer t.log.Tracef("started")
	err := make(chan error)
	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		defer t.log.Infof("stopped")

		t.ctx, t.cancel = context.WithCancel(ctx)
		t.publisher = pubsub.PubFromContext(t.ctx)

		sub := pubsub.SubFromContext(t.ctx, "daemon.istats", t.subQS)
		sub.AddFilter(&msgbus.InstanceConfigDeleted{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.InstanceFrozenFileRemoved{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.InstanceFrozenFileUpdated{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.RunFileRemoved{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.RunFileUpdated{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.InstanceStatusPost{}, t.labelLocalhost)
		sub.Start()
		t.sub = sub

		defer func() {
			if err := sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warnf("subscription stop: %s", err)
			}
		}()
		t.log.Infof("started")
		errC <- nil
		t.worker()
	}(err)

	return <-err
}

func (t *T) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) worker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch msg := i.(type) {
			case *msgbus.InstanceConfigDeleted:
				t.onInstanceConfigDeleted(msg)
			case *msgbus.InstanceFrozenFileRemoved:
				t.onInstanceFrozenFileRemoved(msg)
			case *msgbus.InstanceFrozenFileUpdated:
				t.onInstanceFrozenFileUpdated(msg)
			case *msgbus.RunFileRemoved:
				t.onRunFileDeleted(msg)
			case *msgbus.RunFileUpdated:
				t.onRunFileUpdated(msg)
			case *msgbus.InstanceStatusPost:
				t.onInstanceStatusPost(msg)
			}
		}
	}
}

func (t *T) onInstanceConfigDeleted(msg *msgbus.InstanceConfigDeleted) {
	s := msg.Path.String()
	delete(t.iStatusM, msg.Path.String())
	instance.StatusData.Unset(msg.Path, t.localhost)
	t.publisher.Pub(&msgbus.InstanceStatusDeleted{Path: msg.Path, Node: t.localhost},
		t.labelLocalhost,
		pubsub.Label{"namespace", msg.Path.Namespace},
		pubsub.Label{"path", s},
	)
}

func (t *T) onInstanceFrozenFileRemoved(msg *msgbus.InstanceFrozenFileRemoved) {
	s := msg.Path.String()
	iStatus, ok := t.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if iStatus.FrozenAt.IsZero() {
		// no change
		return
	}
	iStatus.FrozenAt = time.Time{}
	if iStatus.UpdatedAt.Before(msg.At) {
		iStatus.UpdatedAt = msg.At
	}
	t.iStatusM[s] = iStatus
	naming.LogWithPath(t.log, msg.Path).Infof("%s: change frozen to false", s)
	instance.StatusData.Set(msg.Path, t.localhost, iStatus.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceStatusUpdated{Path: msg.Path, Node: t.localhost, Value: *iStatus.DeepCopy()},
		t.labelLocalhost,
		pubsub.Label{"namespace", msg.Path.Namespace},
		pubsub.Label{"path", s},
	)
}

func (t *T) onRunFileUpdated(msg *msgbus.RunFileUpdated) {
	s := msg.Path.String()

	iStatus, ok := t.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if msg.At.Before(iStatus.UpdatedAt) {
		// skip event from past
		return
	}
	if iStatus.Running.Has(msg.RID) {
		return
	}
	runningInfo, err := LoadRunningInfo(msg.RID, msg.File)
	if err != nil {
		t.log.Warnf("%s load running info failed: %s", msg.RID, err)
	} else {
		iStatus.Running = append(iStatus.Running, runningInfo)
	}
	iStatus.UpdatedAt = msg.At
	t.iStatusM[s] = iStatus
	instance.StatusData.Set(msg.Path, t.localhost, iStatus.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceStatusUpdated{Path: msg.Path, Node: t.localhost, Value: *iStatus.DeepCopy()},
		t.labelLocalhost,
		pubsub.Label{"namespace", msg.Path.Namespace},
		pubsub.Label{"path", s},
	)
}

func LoadRunningInfo(rid, file string) (data resource.RunningInfo, errs error) {
	data.RID = rid
	if b, err := os.ReadFile(file); err != nil {
		errors.Join(errs, err)
	} else {
		if sessionID, err := uuid.ParseBytes(b); err != nil {
			errors.Join(errs, err)
		} else {
			data.SessionID = sessionID
		}
	}
	if pid, err := strconv.Atoi(filepath.Base(file)); err != nil {
		errors.Join(errs, err)
	} else {
		data.PID = pid
	}
	return
}

func (t *T) onRunFileDeleted(msg *msgbus.RunFileRemoved) {
	s := msg.Path.String()

	iStatus, ok := t.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if msg.At.Before(iStatus.UpdatedAt) {
		// skip event from past
		return
	}
	if !iStatus.Running.Has(msg.RID) {
		return
	}
	iStatus.Running = slices.DeleteFunc(iStatus.Running, func(data resource.RunningInfo) bool {
		return data.RID == msg.RID
	})
	iStatus.UpdatedAt = msg.At
	t.iStatusM[s] = iStatus
	instance.StatusData.Set(msg.Path, t.localhost, iStatus.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceStatusUpdated{Path: msg.Path, Node: t.localhost, Value: *iStatus.DeepCopy()},
		t.labelLocalhost,
		pubsub.Label{"namespace", msg.Path.Namespace},
		pubsub.Label{"path", s},
	)
}

func (t *T) onInstanceFrozenFileUpdated(msg *msgbus.InstanceFrozenFileUpdated) {
	s := msg.Path.String()

	iStatus, ok := t.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if msg.At.Before(iStatus.FrozenAt) {
		// skip event from past
		return
	}

	iStatus.FrozenAt = msg.At
	if msg.At.After(iStatus.UpdatedAt) {
		iStatus.UpdatedAt = msg.At
	}
	naming.LogWithPath(t.log, msg.Path).Infof("%s: change frozen to true", s)
	t.iStatusM[s] = iStatus
	instance.StatusData.Set(msg.Path, t.localhost, iStatus.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceStatusUpdated{Path: msg.Path, Node: t.localhost, Value: *iStatus.DeepCopy()},
		t.labelLocalhost,
		pubsub.Label{"namespace", msg.Path.Namespace},
		pubsub.Label{"path", s},
	)
}

func (t *T) onInstanceStatusPost(msg *msgbus.InstanceStatusPost) {
	if instance.ConfigData.GetByPathAndNode(msg.Path, t.localhost) == nil {
		return
	}
	s := msg.Path.String()
	prev := t.iStatusM[s]
	if msg.Value.UpdatedAt.Before(prev.UpdatedAt) {
		// Outdated instance status must be ignored: concurrent post from `scheduled status` and `end instance action`
		// This prevents from reverting value until the next scheduled status and fix possible unexpected orchestration
		// --wait failure.
		naming.LogWithPath(t.log, msg.Path).Tracef("%s: ignore outdated %s vs %s", s, msg.Value.UpdatedAt, prev.UpdatedAt)
		return
	}
	if prev.Avail != msg.Value.Avail {
		naming.LogWithPath(t.log, msg.Path).Infof("%s: change avail %s -> %s", s, prev.Avail, msg.Value.Avail)
	}
	if prev.Overall != msg.Value.Overall {
		naming.LogWithPath(t.log, msg.Path).Infof("%s: change overall %s -> %s", s, prev.Overall, msg.Value.Overall)
	}
	if prev.Provisioned != msg.Value.Provisioned {
		naming.LogWithPath(t.log, msg.Path).Infof("%s: change provisioned %s -> %s", s, prev.Provisioned, &msg.Value.Provisioned)
	}
	if prev.IsFrozen() != msg.Value.IsFrozen() {
		naming.LogWithPath(t.log, msg.Path).Infof("%s: change frozen to %v", s, msg.Value.IsFrozen())
	}
	t.iStatusM[s] = msg.Value
	instance.StatusData.Set(msg.Path, msg.Node, msg.Value.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceStatusUpdated{Path: msg.Path, Node: msg.Node, Value: msg.Value},
		t.labelLocalhost,
		pubsub.Label{"namespace", msg.Path.Namespace},
		pubsub.Label{"path", s})
}
