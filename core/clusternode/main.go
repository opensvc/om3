// Package clusternode provides a protected cache for cluster nodes
// It may be used from crm or from daemon
package clusternode

import (
	"sync"
)

var (
	mutex        = sync.RWMutex{}
	clusterNodes = []string{}
)

// Set update cached cluster node list
func Set(l []string) {
	mutex.Lock()
	defer mutex.Unlock()
	clusterNodes = append([]string{}, l...)
}

// Get returns the cached cluster node list
func Get() []string {
	mutex.RLock()
	defer mutex.RUnlock()
	return append([]string{}, clusterNodes...)
}
