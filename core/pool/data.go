package pool

import (
	"sync"
)

type (
	Dataer interface {
		Status
	}

	DataElement[T Dataer] struct {
		Name  string
		Value *T
	}

	// Data defines a shared holder for all pool Dataer
	Data[T Dataer] struct {
		sync.RWMutex
		data map[string]*T
	}
)

var (
	// StatusData is the package data holder for all instances statuses
	StatusData *Data[Status]
)

// Set will add or update instance data
func (c *Data[T]) Set(name string, v *T) {
	c.Lock()
	defer c.Unlock()
	c.data[name] = v
}

// Unset removes an instance data
func (c *Data[T]) Unset(name string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, name)
}

// Get returns a pool data or nil if data is not found
func (c *Data[T]) Get(name string) *T {
	c.RLock()
	v := c.data[name]
	c.RUnlock()
	return v
}

// GetAll returns all instance data as a list of DataElements
func (c *Data[T]) GetAll() []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for name, v := range c.data {
		result = append(result, DataElement[T]{
			Name:  name,
			Value: v,
		})
	}
	c.RUnlock()
	return result
}

func NewData[T Dataer]() *Data[T] {
	return &Data[T]{
		data: make(map[string]*T),
	}
}

// InitData reset package instances data, it can be used for tests.
func InitData() {
	StatusData = NewData[Status]()
}

func init() {
	InitData()
}
