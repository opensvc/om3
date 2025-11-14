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

func (d *T) Close() error {
	return fmt.Errorf("not implemented")
}

func (d *T) NotifyWatchdog() (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (d *T) NotifyReady() (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (d *T) NotifyStopping() (bool, error) {
	return false, fmt.Errorf("not implemented")
}
