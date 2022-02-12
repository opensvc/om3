package zfs

import "strings"

type (
	ZfsName string
)

// String implements the stringer interface.
func (t ZfsName) String() string {
	return string(t)
}

// PoolName returns the pool name extracted from a <pool>/<basename> string.
func (t ZfsName) PoolName() string {
	l := strings.SplitN(string(t), "/", 2)
	return l[0]
}

// BaseName returns the basename extracted from a <pool>/<basename> string.
func (t ZfsName) BaseName() string {
	l := strings.SplitN(string(t), "/", 2)
	return l[1]
}
