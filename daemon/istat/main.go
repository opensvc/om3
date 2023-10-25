// Package istat implements the management of local instance status

package istat

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
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

		log zerolog.Logger

		ctx    context.Context
		cancel context.CancelFunc

		bus *pubsub.Bus
		sub *pubsub.Subscription

		labelLocalhost pubsub.Label
		wg             sync.WaitGroup
	}
)

var (
	// SubscriptionQueueSize is size of "istats" subscription
	SubscriptionQueueSize = 1000
)

func New() *T {
	localhost := hostname.Hostname()
	return &T{
		log:            log.Logger.With().Str("pkg", "istat").Logger(),
		iStatusM:       make(map[string]instance.Status),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
	}
}

func (t *T) Start(ctx context.Context) error {
	err := make(chan error)
	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		defer t.log.Info().Msg("stopped")

		t.ctx, t.cancel = context.WithCancel(ctx)
		t.bus = pubsub.BusFromContext(t.ctx)

		sub := t.bus.Sub("istats", pubsub.WithQueueSize(SubscriptionQueueSize))
		sub.AddFilter(&msgbus.InstanceConfigDeleted{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.InstanceFrozenFileRemoved{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.InstanceFrozenFileUpdated{}, t.labelLocalhost)
		sub.AddFilter(&msgbus.InstanceStatusPost{}, t.labelLocalhost)
		sub.Start()
		t.sub = sub

		defer func() {
			if err := sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warn().Err(err).Msg("subscription stop")
			}
		}()
		t.log.Info().Msg("started")
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
			switch m := i.(type) {
			case *msgbus.InstanceConfigDeleted:
				t.onInstanceConfigDeleted(m)
			case *msgbus.InstanceFrozenFileRemoved:
				t.onInstanceFrozenFileRemoved(m)
			case *msgbus.InstanceFrozenFileUpdated:
				t.onInstanceFrozenFileUpdated(m)
			case *msgbus.InstanceStatusPost:
				t.onInstanceStatusPost(m)
			}
		}
	}
}

func (t *T) onInstanceConfigDeleted(m *msgbus.InstanceConfigDeleted) {
	s := m.Path.String()
	delete(t.iStatusM, m.Path.String())
	instance.StatusData.Unset(m.Path, t.localhost)
	t.bus.Pub(&msgbus.InstanceStatusDeleted{Path: m.Path, Node: t.localhost},
		t.labelLocalhost,
		pubsub.Label{"path", s},
	)
}

func (o *T) onInstanceFrozenFileRemoved(fileRemoved *msgbus.InstanceFrozenFileRemoved) {
	s := fileRemoved.Path.String()
	iStatus, ok := o.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if iStatus.FrozenAt.IsZero() {
		// no change
		return
	}
	iStatus.FrozenAt = time.Time{}
	if iStatus.UpdatedAt.Before(fileRemoved.At) {
		iStatus.UpdatedAt = fileRemoved.At
	}
	o.iStatusM[s] = iStatus
	instance.StatusData.Set(fileRemoved.Path, o.localhost, iStatus.DeepCopy())
	o.bus.Pub(&msgbus.InstanceStatusUpdated{Path: fileRemoved.Path, Node: o.localhost, Value: *iStatus.DeepCopy()},
		o.labelLocalhost,
		pubsub.Label{"path", s},
	)
}

func (o *T) onInstanceFrozenFileUpdated(frozen *msgbus.InstanceFrozenFileUpdated) {
	s := frozen.Path.String()

	iStatus, ok := o.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if frozen.At.Before(iStatus.FrozenAt) {
		// skip event from past
		return
	}

	iStatus.FrozenAt = frozen.At
	if frozen.At.After(iStatus.UpdatedAt) {
		iStatus.UpdatedAt = frozen.At
	}
	o.iStatusM[s] = iStatus
	instance.StatusData.Set(frozen.Path, o.localhost, iStatus.DeepCopy())
	o.bus.Pub(&msgbus.InstanceStatusUpdated{Path: frozen.Path, Node: o.localhost, Value: *iStatus.DeepCopy()},
		o.labelLocalhost,
		pubsub.Label{"path", s},
	)
}

func (o *T) onInstanceStatusPost(post *msgbus.InstanceStatusPost) {
	s := post.Path.String()
	o.iStatusM[s] = post.Value
	instance.StatusData.Set(post.Path, post.Node, post.Value.DeepCopy())
	o.bus.Pub(&msgbus.InstanceStatusUpdated{Path: post.Path, Node: post.Node, Value: post.Value},
		o.labelLocalhost,
		pubsub.Label{"path", s})
}
