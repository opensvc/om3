package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
)

type (
	// dataInstaller is a resource installing data, the keys of stores and
	// directories, under its mount point: a volume or a filesystem.
	dataInstaller interface {
		CanInstall(context.Context) (bool, error)
		RunInstall(context.Context) error
	}
)

// Install installs again what the volumes and the filesystems of the
// instance declare, as their start does, and sends the signals of the files
// it changed, without starting anything.
//
// It says what it leaves as it is too, and why, which a start does not: a
// file up to date, a key the store does not hold, a resource not up here.
func (t *actor) Install(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Install)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("install", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if len(resourceselector.FromContext(ctx, t).Resources()) == 0 {
		t.log.Infof("no resource matches the selection: nothing to install")
		return nil
	}
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return t.installResource(ctx, r)
	})
}

func (t *actor) installResource(ctx context.Context, r resource.Driver) error {
	log := t.log.Attr("rid", r.RID())
	i, ok := r.(dataInstaller)
	if !ok {
		// Said for a resource the user named, and not for every
		// resource of an instance asked whole.
		if actioncontext.HasResourceSelector(ctx) {
			log.Infof("skip %s: it installs nothing", r.RID())
		}
		return nil
	}
	if can, err := i.CanInstall(ctx); err != nil {
		return err
	} else if !can {
		log.Infof("skip %s: it is not up here, and installs on its start", r.RID())
		return nil
	}
	return i.RunInstall(ctx)
}
