// Package muxctx provides functions for mux middlewares or handlers
package muxctx

import (
	"context"
)

type (
	contextKey string
)

var (
	contextMux = contextKey("multiplexed")
)

func (c contextKey) String() string {
	return "muxctx." + string(c)
}

// WithMultiplexed function returns copy of parent with multiplexed bool.
func WithMultiplexed(parent context.Context, multiplexed bool) context.Context {
	return context.WithValue(parent, contextMux, multiplexed)
}

// Multiplexed function returns multiplexed bool from context
func Multiplexed(ctx context.Context) bool {
	multiplexed, ok := ctx.Value(contextMux).(bool)
	if ok {
		return multiplexed
	}
	return false
}
