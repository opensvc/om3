package actionrollback

import (
	"context"
)

type (
	key int

	// T represents a transactional context that maintains a stack of functions
	// to be executed during rollback.
	// Each function in the stack takes a context.Context as input and returns
	// an error.
	T struct {
		stack []func(ctx context.Context) error
	}
)

var (
	tKey key = 0
)

// Rollback iterates over a stack of functions in reverse order and executes
// them.
// If any function returns an error, the rollback halts, and the error is
// returned.
//
// Parameters:
//
//	ctx: The context.Context instance to pass to each function in the stack.
//
// Returns:
//
//		error: Returns an error if any function in the stack fails during execution,
//	        or nil if all functions complete successfully.
func (t *T) Rollback(ctx context.Context) error {
	if t == nil {
		return nil
	}
	n := len(t.stack)
	for i := n - 1; i >= 0; i-- {
		fn := t.stack[i]
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// NewContext initializes a new transactional context and embeds it into the
// provided context.Context.
// This allows associating a rollback stack with a context for managing
// transactional operations.
//
// Parameters:
//
//	ctx: The base context.Context to which the transactional context will be
//	     added.
//
// Returns:
//
//	context.Context: A new context with the transactional context embedded as
//	                 a value.
func NewContext(ctx context.Context) context.Context {
	t := &T{}
	t.stack = make([]func(context.Context) error, 0)
	return context.WithValue(ctx, tKey, t)
}

// FromContext returns a pointer to the transactional context if present,
// or nil if not found.
func FromContext(ctx context.Context) *T {
	v := ctx.Value(tKey)
	if v == nil {
		return nil
	}
	return v.(*T)
}

func Len(ctx context.Context) int {
	t := FromContext(ctx)
	if t == nil {
		return 0
	}
	return len(t.stack)
}

// Register adds a rollback function to the transactional context's stack.
// If the transactional context is not present in the provided context, the
// function does nothing.
//
// Parameters:
//
//	ctx: The context.Context containing the transactional context.
//	fn:  The function to be added to the rollback stack. It takes a
//	     context.Context as input and returns an error during execution.
//
// Behavior:
//   - If a transactional context (T) is found in the given context, the
//     function is added to its stack.
//   - If no transactional context is found, the function does nothing.
func Register(ctx context.Context, fn func(ctx context.Context) error) {
	t := FromContext(ctx)
	if t == nil {
		return
	}
	t.stack = append(t.stack, fn)
}
