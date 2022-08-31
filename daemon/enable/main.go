package enable

import (
	"sync"
)

type (
	T struct {
		*sync.RWMutex
		enabled bool
	}
)

func New() *T {
	return &T{RWMutex: &sync.RWMutex{}}
}

func (t *T) Enable() {
	t.Lock()
	defer t.Unlock()
	t.enabled = true
}

func (t *T) Disable() {
	t.Lock()
	defer t.Unlock()
	t.enabled = false
}

func (t *T) Enabled() bool {
	t.RLock()
	defer t.RUnlock()
	return t.enabled == true
}
