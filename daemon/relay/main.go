package relay

import (
	"strings"
	"sync"
	"time"
)

type (
	Slot struct {
		Value    any
		timer    *time.Timer
		Username string
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

func (m *M) List(username string) []Slot {
	slots := make([]Slot, 0)
	m.Map.Range(func(key, value any) bool {
		slot := value.(Slot)
		if username == "" || slot.Username == username {
			slots = append(slots, slot)
		}
		return true
	})
	return slots
}

func (m *M) Load(username, clusterID, nodename string) (Slot, bool) {
	key := makeRelayKey(username, clusterID, nodename)
	value, ok := m.Map.Load(key)
	if !ok {
		return Slot{}, false
	}
	return value.(Slot), true
}

func (m *M) Store(username, clusterID, nodename string, value any) {
	key := makeRelayKey(username, clusterID, nodename)
	m.stopTimer(key)
	slot := Slot{
		Value: value,
		timer: time.AfterFunc(MaxAge, func() {
			//fmt.Printf("drop key %s, aged %s\n", key, MaxAge)
			m.Map.Delete(key)
		}),
		Username: username,
	}
	m.Map.Store(key, slot)
}

func (m *M) stopTimer(key string) {
	i, ok := m.Map.Load(key)
	if !ok {
		return
	}
	slot := i.(Slot)
	slot.timer.Stop()
}

func makeRelayKey(username, clusterID, nodename string) string {
	return strings.Join([]string{username, clusterID, nodename}, "/")
}
