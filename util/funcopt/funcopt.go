//
// Package funcopt is a functional options helper package.
//
// Functional options beneficits are described at
// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
//
// A typical allocator is implemented as:
//
// func New(opts ...funcopt.O) *T {
//     t := &T{<my defaults>}
//     funcopt.Apply(t, opts...)
//     return t
// }
//
// Example:
//
// func WithName(name string) funcopt.O {
//     return funcopt.F(func(i interface{}) error {
//         t := i.(*T)
//         t.name = name
//         return nil
//     })
// }
//
package funcopt

type (
	// O is the interface that a functional option must implement.
	O interface {
		apply(t interface{}) error
	}

	//
	// F is the prototype of the setter function returned by the
	// option function.
	//
	F func(interface{}) error
)

func (f F) apply(t interface{}) error {
	return f(t)
}

// Apply loops over the functional options and executes their setter
// function.
func Apply(t interface{}, opts ...O) error {
	for _, opt := range opts {
		if err := opt.apply(t); err != nil {
			return err
		}
	}
	return nil
}
