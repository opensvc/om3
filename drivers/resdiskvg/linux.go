//go:build linux

package resdiskvg

import (
	"context"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/lvm2"
)

func (t T) vg() VGDriver {
	vg := lvm2.NewVG(
		t.VGName,
		lvm2.WithLogger(t.Log()),
	)
	return vg
}

func hostTag() string {
	return hostname.Hostname()
}

func (t T) startTag(ctx context.Context) error {
	if err := t.cleanTags(ctx); err != nil {
		return err
	}
	if v, err := t.hasTag(); v || err != nil {
		return err
	}
	if err := t.addTag(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.delTag()
	})
	return nil
}

func (t T) stopTag() error {
	return t.delTag()
}

func (t T) addTag() error {
	return t.vg().AddTag("@" + hostTag())
}

func (t T) hasTag() (bool, error) {
	return t.vg().HasTag(hostTag())
}

func (t T) delTag() error {
	return t.vg().DelTag("@" + hostTag())
}

func (t T) cleanTags(ctx context.Context) error {
	tags, err := t.vg().Tags()
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
		if err := vg.DelTag(tag); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return vg.AddTag(tag)
		})
	}
	return nil
}
