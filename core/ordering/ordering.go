package ordering

type (
	T int
)

const (
	Asc T = iota
	Desc
)

// IsDesc return true if the ordering is descending. Using this produces shorter code.
func (t T) IsDesc() bool {
	return t == Desc
}
