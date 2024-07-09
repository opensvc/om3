package daemonsubsystem

import (
	"sync"
)

type (
	Cacher interface {
		Collector | Dns | Daemondata | Heartbeat | Listener | RunnerImon | Scheduler
	}

	CacheElement[T Cacher] struct {
		Node  string
		Value *T
	}

	// CacheData defines a shared holder for all nodes Cacher
	CacheData[T Cacher] struct {
		sync.RWMutex
		data map[string]*T
	}
)

var (
	// DataCollector is the package data holder for all nodes Collector
	DataCollector *CacheData[Collector]

	// DataDns is the package data holder for all nodes Dns
	DataDns *CacheData[Dns]

	// DataDaemondata is the package data holder for all nodes Daemondata
	DataDaemondata *CacheData[Daemondata]

	// DataHeartbeat is the package data holder for all nodes Heartbeat
	DataHeartbeat *CacheData[Heartbeat]

	// DataListener is the package data holder for all nodes Listener
	DataListener *CacheData[Listener]

	// DataListener is the package data holder for all nodes RunnerImon
	DataRunnerImon *CacheData[RunnerImon]

	// DataScheduler is the package data holder for all nodes Scheduler
	DataScheduler *CacheData[Scheduler]
)

func NewData[T Cacher]() *CacheData[T] {
	return &CacheData[T]{
		data: make(map[string]*T),
	}
}

func DropNode(nodename string) {
	DataCollector.Unset(nodename)
	DataDns.Unset(nodename)
	DataDaemondata.Unset(nodename)
	DataHeartbeat.Unset(nodename)
	DataListener.Unset(nodename)
	DataRunnerImon.Unset(nodename)
	DataScheduler.Unset(nodename)
}

// Set add or update v for nodename
func (c *CacheData[T]) Set(nodename string, v *T) {
	c.Lock()
	defer c.Unlock()
	c.data[nodename] = v
}

// Unset existing stored value for nodename
func (c *CacheData[T]) Unset(nodename string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, nodename)
}

// Get return the stored value for nodename or nil if not found
func (c *CacheData[T]) Get(nodename string) *T {
	c.RLock()
	v := c.data[nodename]
	c.RUnlock()
	return v
}

// GetAll returns all stored elements as list of CacheElement[T]
func (c *CacheData[T]) GetAll() []CacheElement[T] {
	c.RLock()
	result := make([]CacheElement[T], 0)
	for nodename, v := range c.data {
		result = append(result, CacheElement[T]{
			Node:  nodename,
			Value: v,
		})
	}
	c.RUnlock()
	return result
}

// InitData reset package daemondef data, it can be used for tests.
func InitData() {
	DataCollector = NewData[Collector]()
	DataDns = NewData[Dns]()
	DataDaemondata = NewData[Daemondata]()
	DataHeartbeat = NewData[Heartbeat]()
	DataListener = NewData[Listener]()
	DataRunnerImon = NewData[RunnerImon]()
	DataScheduler = NewData[Scheduler]()
}

func init() {
	InitData()
}
