package driverid

import "opensvc.com/opensvc/core/drivergroup"

type (
	Registry map[T]interface{}
)

var (
	registry = NewRegistry()
)

func NewRegistry() Registry {
	return make(Registry)
}

func Register(id T, allocator interface{}) {
	registry[id] = allocator
}

func Exists(id T) bool {
	return Get(id) != nil
}

func Get(id T) interface{} {
	allocator, _ := registry[id]
	return allocator
}

func List() L {
	l := make(L, len(registry))
	i := 0
	for did, _ := range registry {
		l[i] = did
		i = i + 1
	}
	return l
}

func ListGroup(group drivergroup.T) Registry {
	m := NewRegistry()
	for _, did := range List() {
		if did.Group != group {
			continue
		}
		m[did] = Get(did)
	}
	return m
}
