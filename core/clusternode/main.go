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
	clusterNodes = make([]string, len(l))
	copy(clusterNodes, l)
}

// Get returns the cached cluster node list
func Get() []string {
	mutex.RLock()
	defer mutex.RUnlock()
	clusterNodesCopy := make([]string, len(clusterNodes))
	copy(clusterNodesCopy, clusterNodes)
	return clusterNodesCopy
}

// Has returns true if s is a cluster node
func Has(s string) bool {
	for _, e := range Get() {
		if e == s {
			return true
		}
	}
	return false
}
