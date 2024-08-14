package driver

type (
	Registry map[ID]any
)

var (
	All = NewRegistry()
)

func NewRegistry() Registry {
	return make(Registry)
}

func Register(id ID, allocator any) {
	All[id] = allocator
}

func Exists(id ID) bool {
	return Get(id) != nil
}

func Get(id ID) any {
	allocator, ok := All[id]
	if !ok {
		// <group>.<name> driver not found, ... try <group>
		// used for example by the volume driver, whose
		// type keyword is not pointing a resource sub driver
		// but a pool driver.
		id.Name = ""
		return GetStrict(id)
	}
	return allocator
}

func GetStrict(id ID) any {
	allocator, _ := All[id]
	return allocator
}

func List() IDs {
	l := make(IDs, len(All))
	i := 0
	for did := range All {
		l[i] = did
		i = i + 1
	}
	return l
}

func NamesByGroup() map[Group][]string {
	m := make(map[Group][]string)
	for did := range All {
		var l []string
		l, _ = m[did.Group]
		m[did.Group] = append(l, did.Name)
	}
	return m
}

func (t Registry) WithGroup(group Group) Registry {
	m := NewRegistry()
	for did, allocator := range t {
		if did.Group != group {
			continue
		}
		m[did] = allocator
	}
	return m
}
