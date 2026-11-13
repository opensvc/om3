package file

import (
	"bytes"
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

// EditCopy is the copy of the file src an edit is made in, which the edit
// installs once validated, and keeps while it is not, for a --recover.
func EditCopy(src string) string {
	return src + ".tmp"
}

// Edit has the user edit a copy of the file src, and installs it once
// validate accepts it.
//
// The copy is judged against the installed file: a copy identical to it holds
// nothing to recover, so it is not an edit pending, and an edit leaving it
// identical changed nothing. A copy holding changes is kept when the editor
// fails or validate refuses it, for an edit with --recover to go on from it,
// or one with --discard to drop it.
func Edit(src string, mode EditMode, validate func(dst string) error) error {
	dst := EditCopy(src)
	// The copy is a file of its own: a link to the installed file, or to
	// another, would be edited in place of the copy, and judged identical to
	// the installed file with no validation.
	if fi, err := os.Lstat(dst); err == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("the edit copy %s is not a regular file: remove it", dst)
	}
	if Exists(dst) {
		switch {
		case mode == EditModeDiscard:
			if err := os.Remove(dst); err != nil {
				return err
			}
		case mode == EditModeRecover:
		case sameContent(src, dst):
			// Nothing to recover: the edit starts again from it.
		default:
			diff, _ := Diff(src, dst)
			return fmt.Errorf("%w: %s", ErrEditPending, diff)
		}
	}
	if !Exists(dst) {
		if err := Copy(src, dst); err != nil {
			return err
		}
	}
	if err := editor.Edit(dst); err != nil {
		if sameContent(src, dst) {
			_ = os.Remove(dst)
			return err
		}
		return fmt.Errorf("%w: the changes are kept for an edit with --recover", err)
	}
	if sameContent(src, dst) {
		fmt.Println("unchanged")
	} else if err := validate(dst); err != nil {
		return fmt.Errorf("%w: %s: the changes are kept for an edit with --recover", ErrEditValidate, err)
	} else if err := Copy(dst, src); err != nil {
		return err
	}
	return os.Remove(dst)
}

// sameContent says the files a and b hold the same bytes. A file not read
// is not the same.
func sameContent(a, b string) bool {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}
