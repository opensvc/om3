// Package draincommand is a helper for daemon drain commands
package draincommand

import (
	"time"

	"github.com/pkg/errors"
)

type (
	ErrorSetter interface {
		SetError(error)
	}

	// ErrC defines error channel for command
	ErrC chan<- error
)

var (
	ErrDrained = errors.New("drained command")
)

func (c ErrC) SetError(err error) {
	c <- err
}

// Do drains the command chan c
// If pending command is an ErrorSetter, it set cmd error to ErrDrained
func Do(c <-chan any, duration time.Duration) {
	go func() {
		tC := time.After(duration)
		for {
			select {
			case <-tC:
				return
			case i := <-c:
				switch cmd := i.(type) {
				case ErrorSetter:
					cmd.SetError(ErrDrained)
				}
			}
		}
	}()
}
