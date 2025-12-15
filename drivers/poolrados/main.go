package poolrados

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/drivers/resdiskrados"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "rados")
)

func init() {
	driver.Register(drvID, NewPooler)
}

func NewPooler() pool.Pooler {
	t := New()
	var i interface{} = t
	return i.(pool.Pooler)
}

func New() *T {
	t := T{}
	return &t
}

func (t T) Head() string {
	s := t.rbdPool()
	if ns := t.rbdNamespace(); ns != "" {
		s += "/" + ns
	}
	return s
}

func (t T) poolName() string {
	return t.GetString("name")
}

func (t T) rbdPool() string {
	return t.GetString("rbd_pool")
}

func (t T) rbdNamespace() string {
	return t.GetString("rbd_namespace")
}

func (t T) Capabilities() []string {
	return []string{"move", "rox", "rwx", "roo", "rwo", "snap", "blk", "shared"}
}

func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	/*
		{
		  "pools": [
		    {
		      "name": "rbd",
		      "id": 2,
		      "stats": {
		        "stored": 2168995431,
		        "objects": 582,
		        "kb_used": 2118231,
		        "bytes_used": 2169068135,
		        "percent_used": 0.0070989476516842842,
		        "max_avail": 303378759680
		      }
		    }
		  ]
		}
	*/
	rbdPool := t.rbdPool()

	type (
		dfPoolStats struct {
			Stored      int64   `json:"stored"`
			Objects     int     `json:"objects"`
			KBUsed      int64   `json:"kb_used"`
			BytesUsed   int64   `json:"bytes_used"`
			PercentUsed float64 `json:"percent_used"`
			MaxAvail    int64   `json:"max_avail"`
		}
		dfPool struct {
			Name  string      `json:"name"`
			ID    int         `json:"id"`
			Stats dfPoolStats `json:"stats"`
		}
		df struct {
			Pools []dfPool
		}
	)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("ceph"),
		command.WithVarArgs("df", "--format", "json"),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return pool.Usage{}, err
	}
	var data df
	err = json.Unmarshal(b, &data)
	if err != nil {
		return pool.Usage{}, err
	}
	for _, poolData := range data.Pools {
		if poolData.Name == rbdPool {
			usage := pool.Usage{
				Size: poolData.Stats.MaxAvail + poolData.Stats.BytesUsed,
				Free: poolData.Stats.MaxAvail,
				Used: poolData.Stats.BytesUsed,
			}
			return usage, nil
		}
	}
	return pool.Usage{}, fmt.Errorf("pool %s not found", rbdPool)
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	data, err := t.BlkTranslate(name, size, shared)
	if err != nil {
		return nil, err
	}
	data = append(data, t.AddFS(name, shared, 1, 0, "disk#0")...)
	return data, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	rbdPool := t.rbdPool()
	rbdNamespace := t.rbdNamespace()
	rbd := resdiskrados.RBDMap{
		Name:      name,
		Namespace: rbdNamespace,
		Pool:      rbdPool,
	}
	data := []string{
		"disk#0.type=rados",
		"disk#0.name=" + rbd.ImageSpec(),
		"disk#0.size=" + sizeconv.ExactBSizeCompact(float64(size)),
	}
	return data, nil
}
