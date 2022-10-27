package relay

import (
	"strings"
	"sync"
	"time"
)

type (
	capsule struct {
		value any
		timer *time.Timer
	}
	M struct {
		*sync.Map
	}
)

var (
	Map = M{
		Map: &sync.Map{},
	}
	MaxAge = time.Hour * 24
)

func (m *M) Stop() {
	m.Map.Range(func(key, value any) bool {
		m.stopTimer(key.(string))
		m.Map.Delete(key)
		return true
	})
}

func (m *M) Load(clusterID, nodename string) (any, bool) {
	key := makeRelayKey(clusterID, nodename)
	value, ok := m.Map.Load(key)
	if !ok {
		return nil, false
	}
	c := value.(capsule)
	return c.value, true
}

func (m *M) Store(clusterID, nodename string, value any) {
	key := makeRelayKey(clusterID, nodename)
	m.stopTimer(key)
	c := capsule{
		value: value,
		timer: time.AfterFunc(MaxAge, func() {
			//fmt.Printf("drop key %s, aged %s\n", key, MaxAge)
			m.Map.Delete(key)
		}),
	}
	m.Map.Store(key, c)
}

func (m *M) stopTimer(key string) {
	i, ok := m.Map.Load(key)
	if !ok {
		return
	}
	c := i.(capsule)
	c.timer.Stop()
}

func makeRelayKey(clusterID, nodename string) string {
	return strings.Join([]string{clusterID, nodename}, "/")
}
