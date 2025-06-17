package object

import (
	"context"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/file"
)

// Start starts the local instance of the object
func (t *actor) Start(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Start)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("start", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedStart(ctx)
}

func (t *actor) lockedStart(ctx context.Context) error {
	if err := file.Touch(t.lastStartFile(), time.Now()); err != nil {
		return err
	}
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Start(ctx, r)
	})
}

func (t *actor) lastStartFile() string {
	return filepath.Join(t.varDir(), "last_start")
}
