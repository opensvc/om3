package actionrollback

import (
	"context"
	"fmt"
)

type (
	key int
	T   struct {
		stack []func() error
	}
)

var (
	tKey key = 0
)

func NewContext(ctx context.Context) context.Context {
	t := &T{}
	t.stack = make([]func() error, 0)
	return context.WithValue(ctx, tKey, t)
}

func FromContext(ctx context.Context) *T {
	return ctx.Value(tKey).(*T)
}

func Rollback(ctx context.Context) error {
	t := *FromContext(ctx)
	n := len(t.stack)
	for i := n - 1; i >= 0; i-- {
		fn := t.stack[i]
		fmt.Println("pop rollback fn")
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func Register(ctx context.Context, fn func() error) {
	fmt.Println("register rollback")
	t := FromContext(ctx)
	t.stack = append(t.stack, fn)
}
