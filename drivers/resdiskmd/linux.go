//go:build linux

package resdiskmd

import "github.com/opensvc/om3/v3/util/md"

func (t *T) md() MDDriver {
	d := md.New(
		t.GetName(),
		t.UUID,
		md.WithLogger(t.Log()),
	)
	return d
}
