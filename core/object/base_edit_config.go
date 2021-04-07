package object

import (
	"os"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
)

type OptsEditConfig struct {
	Discard bool `flag:"discard"`
	Recover bool `flag:"recover"`
}

func (t Base) EditConfig(opts OptsEditConfig) (err error) {
	src := t.ConfigFile()
	dst := t.editedConfigFile()
	if file.Exists(dst) {
		if opts.Discard {
			if err = os.Remove(dst); err != nil {
				return err
			}
		} else {
			return errors.New(dst + " exists: configuration is already being edited. Set --discard to edit from the current configuration, or --recover to open the unapplied config")
		}
	}
	if opts.Recover {
		if file.Exists(dst) {
			log.Debug().Str("dst", dst).Msg("recover existing configuration temporary copy")
		}
	} else {
		if err = file.Copy(src, dst); err != nil {
			return err
		}
		defer os.Remove(dst)
		log.Debug().Str("dst", dst).Msg("new configuration temporary copy")
	}

	if err = editor.Edit(dst); err != nil {
		return err
	}
	// TODO: Validate dst
	if err = file.Copy(dst, src); err != nil {
		return err
	}
	return nil
}
