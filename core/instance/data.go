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

	deepCopyer[T Dataer] interface {
		DeepCopy() *T
	}
)

var (
	// _ ensures that *Status, *Monitor and *Config implements the deepCopyer[Status] interface.
	_ deepCopyer[Status]  = (*Status)(nil)
	_ deepCopyer[Monitor] = (*Monitor)(nil)
	_ deepCopyer[Config]  = (*Config)(nil)

	// StatusData is the package data holder for all instances statuses
	StatusData *Data[Status]

	// MonitorData is the package data holder for all instances monitors
	MonitorData *Data[Monitor]

	// ConfigData is the package data holder for all instances configs
	ConfigData *Data[Config]
)

// Set will add or update instance data
func (c *Data[T]) Set(p naming.Path, nodename string, v *T) {
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
	c.data[InstanceString(p, nodename)] = v
}

// Unset removes an instance data
func (c *Data[T]) Unset(p naming.Path, nodename string) {
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
	delete(c.data, InstanceString(p, nodename))
}

// DropNode removes node instances
func (c *Data[T]) DropNode(nodename string) {
	c.Lock()
	defer c.Unlock()
	for p := range c.nodeToPath[nodename] {
		delete(c.pathToNode[p], nodename)
		delete(c.data, InstanceString(p, nodename))
	}
	delete(c.nodeToPath, nodename)
}

// GetByPathAndNode returns an instance data or nil if data is not found
func (c *Data[T]) GetByPathAndNode(p naming.Path, nodename string) *T {
	c.RLock()
	defer c.RUnlock()
	return deepCopy(c.data[InstanceString(p, nodename)])
}

// GetByNode returns a map (indexed by path) of instance data for nodename
func (c *Data[T]) GetByNode(nodename string) map[naming.Path]*T {
	c.RLock()
	result := make(map[naming.Path]*T)
	for p := range c.nodeToPath[nodename] {
		result[p] = deepCopy(c.data[InstanceString(p, nodename)])
	}
	c.RUnlock()
	return result
}

// GetByPath returns a map (indexed by nodename) of instance data for path p
func (c *Data[T]) GetByPath(p naming.Path) map[string]*T {
	c.RLock()
	result := make(map[string]*T)
	for nodename := range c.pathToNode[p] {
		result[nodename] = deepCopy(c.data[InstanceString(p, nodename)])
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
				Value: deepCopy(c.data[InstanceString(p, nodename)]),
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
	if StatusData != nil {
		StatusData.Lock()
		defer StatusData.Unlock()
	}
	StatusData = NewData[Status]()

	if MonitorData != nil {
		MonitorData.Lock()
		defer MonitorData.Unlock()
	}
	MonitorData = NewData[Monitor]()

	if ConfigData != nil {
		ConfigData.Lock()
		defer ConfigData.Unlock()
	}
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

func InstanceString(p naming.Path, nodename string) string {
	return p.String() + "@" + nodename
}
