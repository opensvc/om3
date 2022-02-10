package zfs

import (
	"strings"

	"opensvc.com/opensvc/util/funcopt"
)

type (
	ZfsName string
)

func (t ZfsName) String() string {
	return string(t)
}

func (t ZfsName) PoolName() string {
	l := strings.Split(string(t), "/")
	return l[0]
}

func (t *Pool) ListVolumes(fopts ...funcopt.O) (Zvols, error) {
	zvols, err := t.ZfsListVolumes()
	if err != nil {
		return nil, err
	}
	data := make(Zvols, 0)
	for _, zvol := range zvols {
		if zvol.Name.PoolName() != t.Name {
			continue
		}
		data = append(data, zvol)
	}
	return data, nil
}
