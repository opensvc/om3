package funcopt

type (
	O interface {
		apply(t interface{}) error
	}

	F func(interface{}) error
)

func (f F) apply(t interface{}) error {
	return f(t)
}

func Apply(t interface{}, opts ...O) error {
	for _, opt := range opts {
		if err := opt.apply(t); err != nil {
			return err
		}
	}
	return nil
}
