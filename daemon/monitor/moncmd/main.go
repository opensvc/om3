package moncmd

type (
	T interface{}
)

func New(arg interface{}) *T {
	var t T
	t = arg
	return &t
}
