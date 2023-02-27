// Package durationlog
package durationlog

import (
	"context"
	"reflect"
	"time"

	"github.com/rs/zerolog"
)

type (
	T struct {
		Log zerolog.Logger
	}

	stringer interface {
		String() string
	}
)

// WarnExceeded log when delay between <-begin and <-end exceeds maxDuration.
func (t *T) WarnExceeded(ctx context.Context, begin <-chan interface{}, end <-chan bool, maxDuration time.Duration, desc string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var startTime time.Time
	var cmd interface{}
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			tC := time.After(5 * time.Millisecond)
			for {
				select {
				case <-tC:
					return
				case <-begin:
				case <-end:
				case <-ticker.C:
				}
			}
			return
		case c := <-begin:
			startTime = time.Now()
			cmd = c
		case <-end:
			cmd = nil
		case <-ticker.C:
			if cmd != nil && time.Now().Sub(startTime) > maxDuration {
				duration := time.Now().Sub(startTime).Seconds()
				switch c := cmd.(type) {
				case stringer:
					t.Log.Warn().Msgf("max duration exceeded %.02fs: %s: %s", duration, desc, c.String())
				default:
					t.Log.Warn().Msgf("max duration exceeded %.02fs: %s: %s", duration, desc, reflect.TypeOf(cmd).Name())
				}
			}
		}
	}
}
