package cluster

import "sync"

type (
	Dataer interface {
		Config
	}

	// DataT defines a shared holder for all objects Dataer
	DataT[T Dataer] struct {
		sync.RWMutex
		data *T
	}
)

var (
	// ConfigData is the package data holder for local cluster config
	ConfigData *DataT[Config]
)

func NewData[T Dataer]() *DataT[T] {
	return &DataT[T]{}
}

func (c *DataT[T]) IsSet() bool {
	return c.data != nil
}

func (c *DataT[T]) Set(v *T) {
	c.Lock()
	defer c.Unlock()
	c.data = v
}

func (c *DataT[T]) Get() *T {
	c.RLock()
	defer c.RUnlock()
	if c.data == nil {
		panic("Get called before initial Set")
	}
	return c.data
}

// InitData reset package objects data, it can be used for tests.
func InitData() {
	ConfigData = NewData[Config]()
}

func init() {
	InitData()
}
