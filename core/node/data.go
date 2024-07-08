package node

import (
	"sync"

	"github.com/opensvc/om3/util/san"
)

type (
	Gen = map[string]uint64

	Dataer interface {
		Config | Monitor | san.Paths | Stats | Status | Gen
	}

	DataElement[T Dataer] struct {
		Node  string
		Value *T
	}

	// Data defines a shared holder for all nodes Dataer
	Data[T Dataer] struct {
		sync.RWMutex
		data map[string]*T
	}
)

var (
	// ConfigData is the package data holder for all nodes Configs
	ConfigData *Data[Config]

	// MonitorData is the package data holder for all nodes Monitors
	MonitorData *Data[Monitor]

	// OsPathsData is the package data holder for all nodes Os paths data
	OsPathsData *Data[san.Paths]

	// StatsData is the package data holder for all nodes stats
	StatsData *Data[Stats]

	// StatusData is the package data holder for all nodes statuses
	StatusData *Data[Status]

	// GenData is the package data holder for all nodes statuses
	GenData *Data[Gen]
)

func NewData[T Dataer]() *Data[T] {
	return &Data[T]{
		data: make(map[string]*T),
	}
}

// Set add or update v for nodename
func (c *Data[T]) Set(nodename string, v *T) {
	c.Lock()
	defer c.Unlock()
	c.data[nodename] = v
}

// Unset existing stored value for nodename
func (c *Data[T]) Unset(nodename string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, nodename)
}

// Get return the stored value for nodename or nil if not found
func (c *Data[T]) Get(nodename string) *T {
	c.RLock()
	v := c.data[nodename]
	c.RUnlock()
	return v
}

// GetAll returns all stored elements as list of DataElement[T]
func (c *Data[T]) GetAll() []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for nodename, v := range c.data {
		result = append(result, DataElement[T]{
			Node:  nodename,
			Value: v,
		})
	}
	c.RUnlock()
	return result
}

func DropNode(nodename string) {
	ConfigData.Unset(nodename)
	MonitorData.Unset(nodename)
	OsPathsData.Unset(nodename)
	StatusData.Unset(nodename)
	StatsData.Unset(nodename)
	GenData.Unset(nodename)
}

// InitData reset package node data, it can be used for tests.
func InitData() {
	ConfigData = NewData[Config]()
	MonitorData = NewData[Monitor]()
	OsPathsData = NewData[san.Paths]()
	StatusData = NewData[Status]()
	StatsData = NewData[Stats]()
	GenData = NewData[Gen]()
}

func init() {
	InitData()
}
