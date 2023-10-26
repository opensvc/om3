package zfs

import "github.com/opensvc/om3/util/plog"

type (
	Filesystem struct {
		Name string
		Log  *plog.Logger
	}
	Filesystems []Filesystem
)

func (t Filesystem) PoolName() string {
	return ZfsName(t.Name).PoolName()
}

func (t Filesystem) BaseName() string {
	return ZfsName(t.Name).BaseName()
}

func (t Filesystem) GetName() string {
	return t.Name
}

func (t Filesystem) GetLog() *plog.Logger {
	return t.Log
}
