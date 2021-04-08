package client

import "opensvc.com/opensvc/util/funcopt"

type (
	NamespaceSetter interface {
		SetNamespace(string)
	}
	SelectorSetter interface {
		SetSelector(string)
	}
	RelativesSetter interface {
		SetRelatives(bool)
	}
)

// WithNamespace sets a namespace event filter to apply server-side.
func WithNamespace(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		i.(NamespaceSetter).SetNamespace(s)
		return nil
	})
}

// WithSelector sets an object selector event filter to apply server-side.
func WithSelector(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		i.(SelectorSetter).SetSelector(s)
		return nil
	})
}

//
// WithRelatives tells the server to send information about selected objects
// parents, children and slaves.
//
func WithRelatives(s bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		i.(RelativesSetter).SetRelatives(s)
		return nil
	})
}
