//go:build linux

package resdiskvg

import (
	"context"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/lvm2"
)

func (t *T) vg() VGDriver {
	vg := lvm2.NewVG(
		t.VGName,
		lvm2.WithLogger(t.Log()),
	)
	return vg
}

func hostTag() string {
	return hostname.Hostname()
}

func (t *T) startTag(ctx context.Context) error {
	if err := t.cleanTags(ctx); err != nil {
		return err
	}
	if v, err := t.hasTag(ctx); v || err != nil {
		return err
	}
	if err := t.addTag(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.delTag(ctx)
	})
	return nil
}

func (t *T) stopTag(ctx context.Context) error {
	return t.delTag(ctx)
}

func (t *T) addTag(ctx context.Context) error {
	return t.vg().AddTag(ctx, "@"+hostTag())
}

func (t *T) hasTag(ctx context.Context) (bool, error) {
	return t.vg().HasTag(ctx, hostTag())
}

func (t *T) delTag(ctx context.Context) error {
	return t.vg().DelTag(ctx, "@"+hostTag())
}

func (t *T) cleanTags(ctx context.Context) error {
	tags, err := t.vg().Tags(ctx)
	if err != nil {
		return err
	}
	me := hostTag()
	vg := t.vg()
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if tag == me {
			continue
		}
		if err := vg.DelTag(ctx, tag); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return vg.AddTag(ctx, tag)
		})
	}
	return nil
}
