package object

import (
	"context"
	"os"
	"path/filepath"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/timestamp"
)

//
// frozenFile is the path of the file to use as the frozen flag.
// The file mtime is loaded as the frozen key value in the
// instance status dataset.
//
func (t *actor) frozenFile() string {
	return filepath.Join(t.varDir(), "frozen")
}

// Frozen returns the unix timestamp of the last freeze.
func (t *actor) Frozen() timestamp.T {
	p := t.frozenFile()
	fi, err := os.Stat(p)
	if err != nil {
		return timestamp.NewZero()
	}
	return timestamp.New(fi.ModTime())
}

//
// Freeze creates a persistant flag file that prevents orchestration
// of the object instance.
//
func (t *actor) Freeze(ctx context.Context) error {
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	ctx = actioncontext.WithProps(ctx, actioncontext.Freeze)
	defer t.postActionStatusEval(ctx)
	p := t.frozenFile()
	if file.Exists(p) {
		return nil
	}
	d := filepath.Dir(p)
	if !file.Exists(d) {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			return err
		}
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	f.Close()
	t.log.Info().Msg("now frozen")
	return nil
}

//
// Unfreeze removes the persistant flag file that prevents orchestration
// of the object instance.
//
func (t *actor) Unfreeze(ctx context.Context) error {
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	ctx = actioncontext.WithProps(ctx, actioncontext.Unfreeze)
	defer t.postActionStatusEval(ctx)
	p := t.frozenFile()
	if !file.Exists(p) {
		return nil
	}
	err := os.Remove(p)
	if err != nil {
		return err
	}
	t.log.Info().Msg("now unfrozen")
	return nil
}
