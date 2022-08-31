package xconfig

import (
	"fmt"
	"os"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
)

type (
	EditMode int
)

const (
	EditModeNormal EditMode = iota
	EditModeDiscard
	EditModeRecover
)

var (
	ErrEditPending = errors.New(`The configuration is already being edited.
Set --discard to edit from the installed configuration,
or --recover to edit the unapplied config`)
	ErrEditValidate = errors.New("validation errors")
)

func Diff(a, b string) (string, error) {
	var (
		err    error
		ab, bb []byte
	)
	if ab, err = os.ReadFile(a); err != nil {
		return "", err
	}
	if bb, err = os.ReadFile(b); err != nil {
		return "", err
	}
	edits := myers.ComputeEdits(span.URIFromPath(a), string(ab), string(bb))
	return fmt.Sprint(gotextdiff.ToUnified(a, b, string(ab), edits)), nil
}

func Edit(src string, mode EditMode, ref Referrer) error {
	dst := src + ".tmp"
	if file.Exists(dst) {
		switch mode {
		case EditModeDiscard:
			if err := os.Remove(dst); err != nil {
				return err
			}
		case EditModeRecover:
			ref.Log().Debug().Str("dst", dst).Msg("recover existing configuration temporary copy")
		default:
			diff, _ := Diff(src, dst)
			return errors.Wrapf(ErrEditPending, "%s", diff)
		}
	}
	if !file.Exists(dst) {
		if err := file.Copy(src, dst); err != nil {
			return err
		}
		log.Debug().Str("dst", dst).Msg("new configuration temporary copy")
	}
	var refSum []byte
	if b, err := file.MD5(dst); err != nil {
		return err
	} else {
		refSum = b
	}
	if err := editor.Edit(dst); err != nil {
		return err
	}
	if file.HaveSameMD5(refSum, dst) {
		fmt.Println("unchanged")
	} else if err := ValidateFile(dst, ref); err != nil {
		return ErrEditValidate
	} else if err := file.Copy(dst, src); err != nil {
		return err
	}
	if err := os.Remove(dst); err != nil {
		return err
	}
	return nil
}
