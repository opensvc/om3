package zfs

import (
	"opensvc.com/opensvc/util/funcopt"
)

func (t *Pool) ListVolumes(fopts ...funcopt.O) (Vols, error) {
	vols, err := t.ListVolumes()
	if err != nil {
		return nil, err
	}
	data := make(Vols, 0)
	for _, vol := range vols {
		if vol.PoolName() != t.Name {
			continue
		}
		data = append(data, vol)
	}
	return data, nil
}
