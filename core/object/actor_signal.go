package object

import (
	"context"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

type (
	signaler interface {
		Signal(context.Context, syscall.Signal) error
	}
)

func (t *actor) SignalResource(ctx context.Context, rid string, sig syscall.Signal) error {
	t.Resources()
	if r := t.ResourceByID(rid); r == nil {
		return fmt.Errorf("can not find resource %s to send %s to", rid, unix.SignalName(sig))
	} else if s, ok := r.(signaler); !ok {
		return fmt.Errorf("resource %s to send %s to does not support signaling", rid, unix.SignalName(sig))
	} else if err := s.Signal(ctx, sig); err != nil {
		return err
	}
	return nil
}
