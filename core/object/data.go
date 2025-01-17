package object

import (
	"sync"

	"github.com/opensvc/om3/core/naming"
)

type (
	Dataer interface {
		Status
	}

	DataElement[T Dataer] struct {
		Path  naming.Path
		Value *T
	}

	// Data defines a shared holder for all objects Dataer
	Data[T Dataer] struct {
		sync.RWMutex
		data map[naming.Path]*T
	}
)

var (
	// StatusData is the package data holder for all objects statuses
	StatusData *Data[Status]
)

func NewData[T Dataer]() *Data[T] {
	return &Data[T]{
		data: make(map[naming.Path]*T),
	}
}

func (c *Data[T]) Set(p naming.Path, v *T) {
	c.Lock()
	c.data[p] = v
	c.Unlock()
}

func (c *Data[T]) Unset(p naming.Path) {
	c.Lock()
	delete(c.data, p)
	c.Unlock()
}

func (c *Data[T]) GetByPath(p naming.Path) *T {
	c.RLock()
	v := c.data[p]
	c.RUnlock()
	return v
}

func (c *Data[T]) GetAll() []DataElement[T] {
	l := make([]DataElement[T], 0)
	c.RLock()
	for p, v := range c.data {
		l = append(l, DataElement[T]{
			Path:  p,
			Value: v,
		})
	}
	c.RUnlock()
	return l
}

func (c *Data[T]) GetPaths() naming.Paths {
	l := make(naming.Paths, 0)
	c.RLock()
	for k := range c.data {
		l = append(l, k)
	}
	c.RUnlock()
	return l
}

// InitData reset package objects data, it can be used for tests.
func InitData() {
	StatusData = NewData[Status]()
}

func init() {
	InitData()
}
