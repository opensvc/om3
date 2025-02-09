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

	deepCopyer[T Dataer] interface {
		DeepCopy() *T
	}
)

var (
	// _ ensures that *Config implements the deepCopyer[Config] interface.
	_ deepCopyer[Config] = (*Config)(nil)

	// ConfigData is the package data holder for local cluster config
	ConfigData *DataT[Config]
)

func NewData[T Dataer]() *DataT[T] {
	return &DataT[T]{}
}

func (c *DataT[T]) IsSet() bool {
	c.RLock()
	defer c.RUnlock()
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
	return deepCopy(c.data)
}

// InitData reset package objects data, it can be used for tests.
func InitData() {
	ConfigData = NewData[Config]()
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
