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

func (t Base) EditConfig(opts OptsEditConfig) error {
	return editConfig(t.ConfigFile(), opts)
}

func editConfig(src string, opts OptsEditConfig) (err error) {
	var (
		refSum []byte
	)
	dst := src + ".tmp"
	if file.Exists(dst) {
		switch {
		case opts.Discard && opts.Recover:
			return errors.New("discard and recover options are mutually exclusive")
		case opts.Discard:
			if err = os.Remove(dst); err != nil {
				return err
			}
		case opts.Recover:
			log.Debug().Str("dst", dst).Msg("recover existing configuration temporary copy")
		default:
			diff, _ := Diff(src, dst)
			return errors.Wrapf(ErrEditConfigPending, "%s", diff)
		}
	}
	if !file.Exists(dst) {
		if err = file.Copy(src, dst); err != nil {
			return err
		}
		log.Debug().Str("dst", dst).Msg("new configuration temporary copy")
	}
	if refSum, err = file.MD5(dst); err != nil {
		return err
	}
	if err = editor.Edit(dst); err != nil {
		return err
	}
	if file.HaveSameMD5(refSum, dst) {
		fmt.Println("unchanged")
	} else {
		// TODO: Validate dst
		if err = file.Copy(dst, src); err != nil {
			return err
		}
	}
	if err = os.Remove(dst); err != nil {
		return err
	}
	return nil
}
