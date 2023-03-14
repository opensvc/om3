package imon

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/pubsub"
)

func (o *imon) getFrozen() time.Time {
	return file.ModTime(filepath.Join(o.path.VarDir(), "frozen"))
}

func (o *imon) freeze() error {
	p := filepath.Join(o.path.VarDir(), "frozen")
	frozen := o.getFrozen()
	if frozen.After(o.instStatus[o.localhost].Frozen) {
		o.log.Warn().Msgf("freeze is called but frozen file already exists, refresh cache frozen from %s to %s",
			o.instStatus[o.localhost].Frozen, frozen)
		status := o.instStatus[o.localhost]
		status.Frozen = frozen
		o.instStatus[o.localhost] = status
		o.pubsubBus.Pub(msgbus.InstanceFrozenFileUpdated{Path: o.path, Filename: p, Updated: frozen}, pubsub.Label{"path", o.path.String()})
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
	f, err := os.Create(p)
	if err != nil {
		o.log.Error().Err(err).Msg("freeze")
		return err
	}
	_ = f.Close()
	frozen = file.ModTime(p)
	if frozen.IsZero() {
		err := errors.Errorf("unable to retrieve mtime on %s", p)
		o.log.Error().Err(err).Msg("freeze")
		return err
	}
	status := o.instStatus[o.localhost]
	status.Frozen = frozen
	// don't wait for delayed update of local cache
	o.instStatus[o.localhost] = status

	// TODO don't publish and trust discover for this ?
	o.pubsubBus.Pub(msgbus.InstanceFrozenFileUpdated{Path: o.path, Filename: p, Updated: frozen}, pubsub.Label{"path", o.path.String()})
	return nil
}

func (o *imon) unfreeze() error {
	p := filepath.Join(o.path.VarDir(), "frozen")
	if !file.Exists(p) {
		if !o.instStatus[o.localhost].Frozen.Equal(time.Time{}) {
			o.log.Error().Msgf("unfreeze is called but frozen file is absent, clear previous frozen cache from %s",
				o.instStatus[o.localhost].Frozen)
			// don't wait for delayed update of local cache
			// to avoid 'idle -> thawing -> idle -> thawing' until receive local instance status update
			status := o.instStatus[o.localhost]
			status.Frozen = time.Time{}
			o.instStatus[o.localhost] = status

			// TODO don't publish and trust discover for this ?
			o.pubsubBus.Pub(msgbus.InstanceFrozenFileRemoved{Path: o.path, Filename: p, Updated: time.Now()}, pubsub.Label{"path", o.path.String()})
		}
		return nil
	}
	o.log.Info().Msg("daemon action unfreeze")
	now := time.Now()
	err := os.Remove(p)
	if err != nil {
		o.log.Error().Err(err).Msg("unfreeze")
		return err
	}

	// don't wait for delayed update of local cache
	// to avoid 'idle -> thawing -> idle -> thawing' until receive local instance status update
	status := o.instStatus[o.localhost]
	status.Frozen = time.Time{}
	o.instStatus[o.localhost] = status

	// TODO don't publish and trust discover for this ?
	o.pubsubBus.Pub(msgbus.InstanceFrozenFileRemoved{Path: o.path, Filename: p, Updated: now}, pubsub.Label{"path", o.path.String()})
	return nil
}
