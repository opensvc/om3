package object

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
)

type OptsEditConfig struct {
	Discard bool `flag:"discard"`
	Recover bool `flag:"recover"`
}

var ErrEditConfigPending = errors.New(`The configuration is already being edited.
Set --discard to edit from the installed configuration,
or --recover to edit the unapplied config`)

func Diff(a, b string) (string, error) {
	var (
		err    error
		ab, bb []byte
	)
	if ab, err = ioutil.ReadFile(a); err != nil {
		return "", err
	}
	if bb, err = ioutil.ReadFile(b); err != nil {
		return "", err
	}
	edits := myers.ComputeEdits(span.URIFromPath(a), string(ab), string(bb))
	return fmt.Sprint(gotextdiff.ToUnified(a, b, string(ab), edits)), nil
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
			diff, _ := Diff(src, dst)
			return errors.Wrapf(ErrEditConfigPending, "%s", diff)
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
