package stringset

type (
	Set map[string]any
)

func New() Set {
	return make(Set)
}

func (t Set) Slice() []string {
	l := make([]string, len(t))
	i := 0
	for k := range t {
		l[i] = k
		i += 1
	}
	return l
}

func (t Set) Add(l ...string) {
	for _, s := range l {
		t[s] = nil
	}
}

func (t Set) Remove(l ...string) {
	for _, s := range l {
		delete(t, s)
	}
}

func (t Set) Contains(s string) bool {
	_, ok := t[s]
	return ok
}
