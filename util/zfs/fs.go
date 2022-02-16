package zfs

import (
	"github.com/rs/zerolog"
)

type (
	Filesystem struct {
		Name string
		Log  *zerolog.Logger
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

func (t Filesystem) GetLog() *zerolog.Logger {
	return t.Log
}
