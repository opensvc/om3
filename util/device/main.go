package device

import (
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		path string
		log  *zerolog.Logger
	}
)

func New(path string, opts ...funcopt.O) *T {
	t := T{
		path: path,
	}
	_ = funcopt.Apply(&t, opts...)
	return &t
}

func WithLogger(log *zerolog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func (t T) String() string {
	return t.path
}

func (t T) Path() string {
	return t.path
}

func (t *T) RemoveHolders() error {
	return RemoveHolders(t)
}

func RemoveHolders(head *T) error {
	holders, err := head.Holders()
	if err != nil {
		return err
	}
	for _, dev := range holders {
		if err := RemoveHolders(dev); err != nil {
			return err
		}
		if err := dev.Remove(); err != nil {
			return err
		}
	}
	return nil
}
