package zfs

import "strings"

type (
	DatasetName string
)

// String implements the stringer interface.
func (t DatasetName) String() string {
	return string(t)
}

// PoolName returns the pool name extracted from a <pool>/<basename> string.
func (t DatasetName) PoolName() string {
	l := strings.SplitN(string(t), "/", 2)
	return l[0]
}

// BaseName returns the basename extracted from a <pool>/<basename> string.
func (t DatasetName) BaseName() string {
	l := strings.SplitN(string(t), "/", 2)
	if len(l) < 2 {
		return ""
	}
	return l[1]
}
