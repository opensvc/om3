package zfs

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	Vol struct {
		Name      string
		Size      uint64
		BlockSize uint64
		Log       *plog.Logger
	}
	Vols []Vol
)

func (t Vol) PoolName() string {
	return DatasetName(t.Name).PoolName()
}

func (t Vol) BaseName() string {
	return DatasetName(t.Name).BaseName()
}

func (t Vol) GetName() string {
	return t.Name
}

func (t Vol) GetLog() *plog.Logger {
	return t.Log
}

func (t Vols) Paths() []string {
	l := make([]string, 0)
	for _, vol := range t {
		p := fmt.Sprintf("/dev/%s/%s", vol.PoolName(), vol.BaseName())
		l = append(l, p)
	}
	return l
}
