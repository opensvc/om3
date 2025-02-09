package node

import (
	"sync"

	"github.com/opensvc/om3/util/san"
)

type (
	Gen map[string]uint64

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

	deepCopyer[T Dataer] interface {
		DeepCopy() *T
	}
)

var (
	// _ ensures implements the deepCopyer[] interface.
	_ deepCopyer[Config]    = (*Config)(nil)
	_ deepCopyer[Monitor]   = (*Monitor)(nil)
	_ deepCopyer[san.Paths] = (*san.Paths)(nil)
	_ deepCopyer[Stats]     = (*Stats)(nil)
	_ deepCopyer[Status]    = (*Status)(nil)
	_ deepCopyer[Gen]       = (*Gen)(nil)

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

// GetByNode return the stored value for nodename or nil if not found
func (c *Data[T]) GetByNode(nodename string) *T {
	c.RLock()
	defer c.RUnlock()
	return deepCopy(c.data[nodename])
}

// GetAll returns all stored elements as list of DataElement[T]
func (c *Data[T]) GetAll() []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for nodename, v := range c.data {
		result = append(result, DataElement[T]{
			Node:  nodename,
			Value: deepCopy(v),
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

func (t *Gen) DeepCopy() *Gen {
	r := make(Gen)
	for k, v := range *t {
		r[k] = v
	}
	return &r
}

func deepCopy[T Dataer](t *T) *T {
	if t == nil {
		return t
	}
	var i any = t
	return i.(deepCopyer[T]).DeepCopy()
}

func init() {
	InitData()
}
