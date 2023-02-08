package zfs

import (
	"github.com/opensvc/om3/util/funcopt"
)

func (t *Pool) ListVolumes(fopts ...funcopt.O) (Vols, error) {
	fopts = append(fopts, ListDatasetsWithLogger(t.Log))
	vols, err := ListVolumes(fopts...)
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
