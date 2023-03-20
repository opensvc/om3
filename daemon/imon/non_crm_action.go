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

// freeze creates missing instance frozen flag file, and publish InstanceFrozenFileUpdated
// local instance status cache frozen value is updated with value read from file system
func (o *imon) freeze() error {
	frozen := o.getFrozen()

	o.log.Info().Msg("daemon action freeze")
	p := filepath.Join(o.path.VarDir(), "frozen")

	if !file.Exists(p) {
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
	}
	frozen = file.ModTime(p)
	if instanceStatus, ok := o.instStatus[o.localhost]; ok {
		instanceStatus.Frozen = frozen
		o.instStatus[o.localhost] = instanceStatus
	}
	if frozen.IsZero() {
		err := errors.Errorf("unexpected frozen reset on %s", p)
		o.log.Error().Err(err).Msg("freeze")
		return err
	}
	o.pubsubBus.Pub(msgbus.InstanceFrozenFileUpdated{Path: o.path, Updated: frozen},
		pubsub.Label{"node", o.localhost},
		pubsub.Label{"path", o.path.String()},
	)
	return nil
}

// freeze removes instance frozen flag file, and publish InstanceFrozenFileUpdated
// local instance status cache frozen value is updated with value read from file system
func (o *imon) unfreeze() error {
	o.log.Info().Msg("daemon action unfreeze")
	p := filepath.Join(o.path.VarDir(), "frozen")
	if !file.Exists(p) {
		o.log.Info().Msg("already thawed")
	} else {
		err := os.Remove(p)
		if err != nil {
			o.log.Error().Err(err).Msg("unfreeze")
			return err
		}
	}
	if instanceStatus, ok := o.instStatus[o.localhost]; ok {
		instanceStatus.Frozen = time.Time{}
		o.instStatus[o.localhost] = instanceStatus
	}
	o.pubsubBus.Pub(msgbus.InstanceFrozenFileRemoved{Path: o.path, Updated: time.Now()},
		pubsub.Label{"node", o.localhost},
		pubsub.Label{"path", o.path.String()},
	)
	return nil
}
