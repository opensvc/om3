package smon

import (
	"os"
	"path/filepath"
	"time"

	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/util/file"
)

func (o *smon) freeze() error {
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
	f, err := os.Create(p)
	if err != nil {
		o.log.Error().Err(err).Msg("freeze")
		return err
	}
	f.Close()
	status := o.instStatus[o.localhost]
	now := time.Now()
	status.Frozen = now
	if err := daemondata.SetInstanceFrozen(o.dataCmdC, o.path, now); err != nil {
		o.log.Warn().Err(err).Msgf("can't set instance status frozen for %s", p)
		return err
	}
	return nil
}

func (o *smon) unfreeze() error {
	p := filepath.Join(o.path.VarDir(), "frozen")
	if !file.Exists(p) {
		return nil
	}
	o.log.Info().Msg("daemon action unfreeze")
	err := os.Remove(p)
	if err != nil {
		o.log.Error().Err(err).Msg("unfreeze")
		return err
	}
	status := o.instStatus[o.localhost]
	status.Frozen = time.Time{}
	if err := daemondata.SetInstanceFrozen(o.dataCmdC, o.path, time.Time{}); err != nil {
		o.log.Warn().Err(err).Msgf("can't set instance status frozen for %s", p)
		return err
	}
	return nil
}
