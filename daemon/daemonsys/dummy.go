//go:build !linux

package daemonsys

import (
	"context"
	"fmt"
)

type (
	T struct{}
)

func New(_ context.Context) (*T, error) {
	return nil, fmt.Errorf("not implemented")
}
