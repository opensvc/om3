package enable

import (
	"sync"
)

type (
	T struct {
		enabled bool
		lock    *sync.RWMutex
	}
)

func New() *T {
	return &T{lock: &sync.RWMutex{}}
}

func (t *T) Enable() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.enabled = true
}

func (t *T) Disable() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.enabled = false
}

func (t *T) Enabled() bool {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.enabled == true
}
