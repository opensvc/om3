package daemondata

import "context"

type contextKey int

const busContextKey contextKey = 0

// BusFromContext function returns the command chan stored in the context
func BusFromContext(ctx context.Context) chan<- Caller {
	cmdC, ok := ctx.Value(busContextKey).(chan<- Caller)
	if ok {
		return cmdC
	}
	panic("unable to retrieve daemon data command chan from context")
}

// ContextWithBus function returns copy of parent, including the daemon data command chan.
func ContextWithBus(parent context.Context, cmd chan<- Caller) context.Context {
	return context.WithValue(parent, busContextKey, cmd)
}

func FromContext(ctx context.Context) *T {
	bus := BusFromContext(ctx)
	return New(bus)
}
