// Package bootid provides node boot id.
//
// Once computed, the value returned by Get() is a cached value until Scan() is
// called again.
package bootid

import (
	"sync"
)

type (
	bootID struct {
		sync.RWMutex
		value string
	}
)

var (
	once   = sync.Once{}
	hostID *bootID
)

// Get returns the node boot id cached value
//
// First call will automatically populate cache.
func Get() string {
	once.Do(func() {
		hostID = &bootID{}
		if s, err := scan(); err != nil {
			// Ignore scan errors, the boot id value returned will be ""
		} else {
			hostID.set(s)
		}
	})
	return hostID.get()
}

// Set may be used to set alternate boot id value instead of automatically scanned value
func Set(s string) {
	once.Do(func() {
		hostID = &bootID{}
	})
	hostID.set(s)
}

// Scan may be used to recompute node boot id value. It also updates cache when
// scan is successful.
func Scan() (s string, err error) {
	once.Do(func() {
		hostID = &bootID{}
	})
	s, err = scan()
	if err != nil {
		return
	}
	hostID.set(s)
	return
}

func (v *bootID) get() string {
	v.RLock()
	defer v.RUnlock()
	return v.value
}

func (v *bootID) set(s string) {
	v.Lock()
	defer v.Unlock()
	v.value = s
}

func (v *bootID) refresh() error {
	s, err := scan()
	if err != nil {
		return err
	}
	v.Lock()
	defer v.Unlock()
	v.value = s
	return nil
}
