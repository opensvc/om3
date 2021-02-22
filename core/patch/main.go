package patch

// Type describes a opensvc dataset change
type Type []interface{}

// SetType is a list of patches
type SetType []Type

// New allocates and initializes a patch
func New(data []interface{}) Type {
	p := make(Type, 0)
	for _, v := range data {
		p = append(p, v)
	}
	return p
}

// NewSet allocates and initializes a patchset
func NewSet(data []interface{}) SetType {
	ps := make(SetType, 0)
	for _, v := range data {
		ps = append(ps, New(v.([]interface{})))
	}
	return ps
}
