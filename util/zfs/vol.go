package zfs

import (
	"fmt"

	"github.com/rs/zerolog"
)

type (
	Vol struct {
		Name      string
		Size      uint64
		BlockSize uint64
		Log       *zerolog.Logger
		LogPrefix string
	}
	Vols []Vol
)

func (t Vol) PoolName() string {
	return ZfsName(t.Name).PoolName()
}

func (t Vol) BaseName() string {
	return ZfsName(t.Name).BaseName()
}

func (t Vol) GetName() string {
	return t.Name
}

func (t Vol) GetLog() *zerolog.Logger {
	return t.Log
}

func (t Vol) GetLogPrefix() string {
	return t.LogPrefix
}

func (t Vols) Paths() []string {
	l := make([]string, 0)
	for _, vol := range t {
		p := fmt.Sprintf("/dev/%s/%s", vol.PoolName(), vol.BaseName())
		l = append(l, p)
	}
	return l
}
