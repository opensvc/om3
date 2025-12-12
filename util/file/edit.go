package file

import (
	"errors"
	"fmt"
	"os"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/opensvc/om3/v3/util/editor"
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
	ErrEditPending  = errors.New(`the configuration is already being edited (--discard to edit from the installed configuration or --recover to edit the unapplied config)`)
	ErrEditValidate = errors.New("configuration validation error")
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

func Edit(src string, mode EditMode, validate func(dst string) error) error {
	dst := src + ".tmp"
	if Exists(dst) {
		switch mode {
		case EditModeDiscard:
			if err := os.Remove(dst); err != nil {
				return err
			}
		case EditModeRecover:
		default:
			diff, _ := Diff(src, dst)
			return fmt.Errorf("%w: %s", ErrEditPending, diff)
		}
	}
	if !Exists(dst) {
		if err := Copy(src, dst); err != nil {
			return err
		}
		//log.Debug().Str("dst", dst).Msg("new configuration temporary copy")
	}
	var refSum []byte
	if b, err := MD5(dst); err != nil {
		return err
	} else {
		refSum = b
	}
	if err := editor.Edit(dst); err != nil {
		return err
	}
	if HaveSameMD5(refSum, dst) {
		fmt.Println("unchanged")
	} else if err := validate(dst); err != nil {
		return fmt.Errorf("%w: %s", ErrEditValidate, err)
	} else if err := Copy(dst, src); err != nil {
		return err
	}
	if err := os.Remove(dst); err != nil {
		return err
	}
	return nil
}
