package imon

import (
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/pubsub"
)

func (o *imon) freeze() error {
	p := filepath.Join(o.path.VarDir(), "frozen")
	if file.Exists(p) {
		return nil
	}
	o.log.Info().Msg("daemon action freeze")
	d := filepath.Dir(p)
	if !file.Exists(d) {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			o.log.Error().Err(err).Msg("freeze")
			return err
		}
	}
	now := time.Now()
	f, err := os.Create(p)
	if err != nil {
		o.log.Error().Err(err).Msg("freeze")
		return err
	}
	_ = f.Close()
	status := o.instStatus[o.localhost]
	status.Frozen = now

	// TODO don't publish and trust discover for this ?
	o.pubsubBus.Pub(msgbus.InstanceFrozenFileUpdated{Path: o.path, Filename: p, Updated: now}, pubsub.Label{"path", o.path.String()})

	// don't wait for delayed update of local cache
	o.instStatus[o.localhost] = status
	return nil
}

func (o *imon) unfreeze() error {
	p := filepath.Join(o.path.VarDir(), "frozen")
	if !file.Exists(p) {
		return nil
	}
	o.log.Info().Msg("daemon action unfreeze")
	now := time.Now()
	err := os.Remove(p)
	if err != nil {
		o.log.Error().Err(err).Msg("unfreeze")
		return err
	}
	// TODO don't publish and trust discover for this ?
	o.pubsubBus.Pub(msgbus.InstanceFrozenFileRemoved{Path: o.path, Filename: p, Updated: now}, pubsub.Label{"path", o.path.String()})
	status := o.instStatus[o.localhost]
	status.Frozen = time.Time{}
	// don't wait for delayed update of local cache
	// to avoid 'idle -> thawing -> idle -> thawing' until receive local instance status update
	o.instStatus[o.localhost] = status
	return nil
}
