// +build linux

package resdiskmd

import "opensvc.com/opensvc/util/md"

func (t T) md() MDDriver {
	d := md.New(
		t.Name(),
		t.UUID,
		md.WithLogger(t.Log()),
	)
	return d
}
