package instance

import (
	"sync"

	"github.com/opensvc/om3/core/naming"
)

type (
	Dataer interface {
		Status | Monitor | Config
	}

	DataElement[T Dataer] struct {
		Path  naming.Path
		Node  string
		Value *T
	}

	// Data defines a shared holder for all instances Dataer
	Data[T Dataer] struct {
		sync.RWMutex
		nodeToPath map[string]map[naming.Path]struct{}
		pathToNode map[naming.Path]map[string]struct{}
		data       map[string]*T
	}
)

var (
	// StatusData is the package data holder for all instances statuses
	StatusData *Data[Status]

	// MonitorData is the package data holder for all instances monitors
	MonitorData *Data[Monitor]

	// ConfigData is the package data holder for all instances configs
	ConfigData *Data[Config]
)

// Set will add or update instance data
func (c *Data[T]) Set(p naming.Path, nodename string, v *T) {
	id := p.String() + "@" + nodename
	c.Lock()
	defer c.Unlock()
	if _, ok := c.nodeToPath[nodename]; !ok {
		c.nodeToPath[nodename] = make(map[naming.Path]struct{})
	}
	if _, ok := c.pathToNode[p]; !ok {
		c.pathToNode[p] = make(map[string]struct{})
	}
	c.nodeToPath[nodename][p] = struct{}{}
	c.pathToNode[p][nodename] = struct{}{}
	c.data[id] = v
}

// Unset removes an instance data
func (c *Data[T]) Unset(p naming.Path, nodename string) {
	id := p.String() + "@" + nodename
	c.Lock()
	defer c.Unlock()
	delete(c.nodeToPath[nodename], p)
	if len(c.nodeToPath[nodename]) == 0 {
		delete(c.nodeToPath, nodename)
	}
	delete(c.pathToNode[p], nodename)
	if len(c.pathToNode[p]) == 0 {
		delete(c.pathToNode, p)
	}
	delete(c.data, id)
}

// DropNode removes node instances
func (c *Data[T]) DropNode(nodename string) {
	c.Lock()
	defer c.Unlock()
	for p := range c.nodeToPath[nodename] {
		delete(c.pathToNode[p], nodename)
		delete(c.data, p.String()+"@"+nodename)
	}
	delete(c.nodeToPath, nodename)
}

// Get returns an instance data or nil if data is not found
func (c *Data[T]) Get(p naming.Path, nodename string) *T {
	id := p.String() + "@" + nodename
	c.RLock()
	v := c.data[id]
	c.RUnlock()
	return v
}

// GetByNode returns a map (indexed by path) of instance data for nodename
func (c *Data[T]) GetByNode(nodename string) map[naming.Path]*T {
	c.RLock()
	result := make(map[naming.Path]*T)
	for p := range c.nodeToPath[nodename] {
		result[p] = c.data[p.String()+"@"+nodename]
	}
	c.RUnlock()
	return result
}

// GetByPath returns a map (indexed by nodename) of instance data for path p
func (c *Data[T]) GetByPath(p naming.Path) map[string]*T {
	c.RLock()
	result := make(map[string]*T)
	for nodename := range c.pathToNode[p] {
		result[nodename] = c.data[p.String()+"@"+nodename]
	}
	c.RUnlock()
	return result
}

// GetAll returns all instance data as a list of DataElements
func (c *Data[T]) GetAll() []DataElement[T] {
	c.RLock()
	result := make([]DataElement[T], 0)
	for nodename, v := range c.nodeToPath {
		for p := range v {
			result = append(result, DataElement[T]{
				Path:  p,
				Node:  nodename,
				Value: c.data[p.String()+"@"+nodename],
			})
		}
	}
	c.RUnlock()
	return result
}

func NewData[T Dataer]() *Data[T] {
	return &Data[T]{
		nodeToPath: make(map[string]map[naming.Path]struct{}),
		pathToNode: make(map[naming.Path]map[string]struct{}),
		data:       make(map[string]*T),
	}
}

func DropNode(nodename string) {
	MonitorData.DropNode(nodename)
	ConfigData.DropNode(nodename)
	StatusData.DropNode(nodename)
}

// InitData reset package instances data, it can be used for tests.
func InitData() {
	StatusData = NewData[Status]()
	MonitorData = NewData[Monitor]()
	ConfigData = NewData[Config]()
}

func init() {
	InitData()
}
