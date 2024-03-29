package object

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

type (
	signaler interface {
		Signal(syscall.Signal) error
	}
)

func (t *actor) SignalResource(rid string, sig syscall.Signal) error {
	t.Resources()
	r := t.ResourceByID(rid)
	if r == nil {
		return fmt.Errorf("can not find resource %s to send %s to", rid, unix.SignalName(sig))
	}
	var (
		i  interface{} = r
		s  signaler
		ok bool
	)
	if s, ok = i.(signaler); !ok {
		return fmt.Errorf("resource %s to send %s to does not support signaling", rid, unix.SignalName(sig))
	}
	if err := s.Signal(sig); err != nil {
		return err
	}
	return nil
}
