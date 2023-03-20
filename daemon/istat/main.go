// Package istat implement management of local instance status

package istat

import (
	"context"
	"errors"
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

		ctx context.Context

		bus *pubsub.Bus
		sub *pubsub.Subscription

		labelLocalhost pubsub.Label
	}
)

func Start(ctx context.Context) error {
	localhost := hostname.Hostname()
	t := T{
		ctx: ctx,
		bus: pubsub.BusFromContext(ctx),
		log: log.Logger.With().Str("func", "istat").Logger(),

		iStatusM:       make(map[string]instance.Status),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
	}

	sub := t.bus.Sub("istats")
	sub.AddFilter(msgbus.InstanceConfigDeleted{}, t.labelLocalhost)
	sub.AddFilter(msgbus.InstanceFrozenFileRemoved{}, t.labelLocalhost)
	sub.AddFilter(msgbus.InstanceFrozenFileUpdated{}, t.labelLocalhost)
	sub.AddFilter(msgbus.InstanceStatusPost{}, t.labelLocalhost)
	sub.Start()
	t.sub = sub

	started := make(chan error)
	go func() {
		t.log.Info().Msg("started")
		started <- nil
		defer func() {
			if err := sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warn().Err(err).Msg("subscription stop")
			}
			t.log.Info().Msg("done")
		}()

		t.worker()
	}()

	return <-started
}

func (t *T) worker() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch m := i.(type) {
			case msgbus.InstanceConfigDeleted:
				t.onInstanceConfigDeleted(m)
			case msgbus.InstanceFrozenFileRemoved:
				t.onInstanceFrozenFileRemoved(m)
			case msgbus.InstanceFrozenFileUpdated:
				t.onInstanceFrozenFileUpdated(m)
			case msgbus.InstanceStatusPost:
				t.onInstanceStatusPost(m)
			}
		}
	}
}

func (t *T) onInstanceConfigDeleted(m msgbus.InstanceConfigDeleted) {
	s := m.Path.String()
	delete(t.iStatusM, m.Path.String())
	t.bus.Pub(msgbus.InstanceStatusDeleted{Path: m.Path, Node: t.localhost},
		t.labelLocalhost,
		pubsub.Label{"path", s},
	)
}

func (o *T) onInstanceFrozenFileRemoved(fileRemoved msgbus.InstanceFrozenFileRemoved) {
	s := fileRemoved.Path.String()
	iStatus, ok := o.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if iStatus.Frozen.IsZero() {
		// no change
		return
	}
	iStatus.Frozen = time.Time{}
	if iStatus.Updated.Before(fileRemoved.Updated) {
		iStatus.Updated = fileRemoved.Updated
	}
	o.iStatusM[s] = iStatus
	o.bus.Pub(msgbus.InstanceStatusUpdated{Path: fileRemoved.Path, Node: o.localhost, Value: *iStatus.DeepCopy()},
		o.labelLocalhost,
		pubsub.Label{"path", s},
	)
}

func (o *T) onInstanceFrozenFileUpdated(frozen msgbus.InstanceFrozenFileUpdated) {
	s := frozen.Path.String()

	iStatus, ok := o.iStatusM[s]
	if !ok {
		// no instance status to update
		return
	}
	if frozen.Updated.Before(iStatus.Frozen) {
		// skip event from past
		return
	}

	iStatus.Frozen = frozen.Updated
	if frozen.Updated.After(iStatus.Updated) {
		iStatus.Updated = frozen.Updated
	}
	o.iStatusM[s] = iStatus
	o.bus.Pub(msgbus.InstanceStatusUpdated{Path: frozen.Path, Node: o.localhost, Value: *iStatus.DeepCopy()},
		o.labelLocalhost,
		pubsub.Label{"path", s},
	)
}

func (o *T) onInstanceStatusPost(post msgbus.InstanceStatusPost) {
	s := post.Path.String()
	o.iStatusM[s] = *post.Value.DeepCopy()
	o.bus.Pub(msgbus.InstanceStatusUpdated{Path: post.Path, Node: post.Node, Value: post.Value},
		o.labelLocalhost,
		pubsub.Label{"path", s})
}
