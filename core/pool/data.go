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
		Node  string
		Value *T
	}

	// Data defines a shared holder for all pool Dataer
	Data[T Dataer] struct {
		sync.RWMutex
		data map[[2]string]*T
	}

	deepCopyer[T Dataer] interface {
		DeepCopy() *T
	}
)

var (
	// _ ensures that *Status implements the deepCopyer[Status] interface.
	_ deepCopyer[Status] = (*Status)(nil)

	// StatusData is the package data holder for all instances statuses
	StatusData *Data[Status]
)

func (c *Data[T]) index(name, node string) [2]string {
	return [2]string{name, node}
}

// Set will add or update instance data
func (c *Data[T]) Set(name, node string, v *T) {
	c.Lock()
	defer c.Unlock()
	c.data[c.index(name, node)] = v
}

// Unset removes an instance data
func (c *Data[T]) Unset(name, node string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, c.index(name, node))
}

// Get returns a pool data or nil if data is not found
func (c *Data[T]) Get(name, node string) *T {
	c.RLock()
	defer c.RUnlock()
	return deepCopy(c.data[c.index(name, node)])
}

// GetAll returns all pool data as a list of DataElements
func (c *Data[T]) GetAll() []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for index, v := range c.data {
		result = append(result, DataElement[T]{
			Name:  index[0],
			Node:  index[1],
			Value: deepCopy(v),
		})
	}
	c.RUnlock()
	return result
}

// GetByName returns pool instances on a specific node as a list of DataElements
func (c *Data[T]) GetByName(name string) []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for index, v := range c.data {
		if index[0] != name {
			continue
		}
		result = append(result, DataElement[T]{
			Name:  index[0],
			Node:  index[1],
			Value: deepCopy(v),
		})
	}
	c.RUnlock()
	return result
}

// GetByNode returns pool instances on a specific node as a list of DataElements
func (c *Data[T]) GetByNode(nodename string) []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for index, v := range c.data {
		if index[1] != nodename {
			continue
		}
		result = append(result, DataElement[T]{
			Name:  index[0],
			Node:  index[1],
			Value: deepCopy(v),
		})
	}
	c.RUnlock()
	return result
}

func NewData[T Dataer]() *Data[T] {
	return &Data[T]{
		data: make(map[[2]string]*T),
	}
}

// InitData reset package instances data, it can be used for tests.
func InitData() {
	StatusData = NewData[Status]()
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
