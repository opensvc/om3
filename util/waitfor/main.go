package waitfor

import (
	"context"
	"time"
)

// Bool waits for fn() to return the specified expected result (either true or
// false) within a defined timeout period, checking at each interval.
func Bool(expected bool, timeout, interval time.Duration, fn func() bool) bool {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if fn() == expected {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// BoolNoError waits for fn() to return the specified expected result (either true
// or false) without error within a defined timeout period, checking at each
// interval.
//
//	returns:
//	   false, err: on first fn() with non nil error
//	   true, nil: on first fn() that returns true, nil
//	   false, nil: when timeout is reached
func BoolNoError(expected bool, timeout, interval time.Duration, fn func() (bool, error)) (bool, error) {
	return BoolNoErrorCtx(context.Background(), expected, timeout, interval, fn)
}

// True waits for fn() to return true within a defined timeout period, checking
// at each interval.
func True(timeout, interval time.Duration, fn func() bool) bool {
	return Bool(true, timeout, interval, fn)
}

// TrueNoError is BoolNoError with expected value true
func TrueNoError(timeout, interval time.Duration, fn func() (bool, error)) (bool, error) {
	return BoolNoError(true, timeout, interval, fn)
}

// False waits for fn() to return false within a defined timeout period,
// checking at each interval.
func False(timeout, interval time.Duration, fn func() bool) bool {
	return Bool(false, timeout, interval, fn)
}

// FalseNoError is BoolNoError with expected value false
func FalseNoError(timeout, interval time.Duration, fn func() (bool, error)) (bool, error) {
	return BoolNoError(false, timeout, interval, fn)
}

// Boolctx waits for fn() to return the specified expected result (either true
// or false) while context (with timeout if timeout > 0) is not done or
// canceled, checking at each interval.
func BoolCtx(ctx context.Context, expected bool, timeout, interval time.Duration, fn func() bool) bool {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		select {
		case <-ctx.Done():
			return false
		default:
			if fn() == expected {
				return true
			}
			time.Sleep(interval)
		}
	}
}

// TrueCtx waits for fn() to return true while context (with timeout if timeout > 0)
// is not done or canceled, checking at each interval.
func TrueCtx(ctx context.Context, timeout, interval time.Duration, fn func() bool) bool {
	return BoolCtx(ctx, true, timeout, interval, fn)
}

// FalseCtx waits for fn() to return false while context (with timeout if timeout > 0)
// is not done or canceled, checking at each interval.
func FalseCtx(ctx context.Context, timeout, interval time.Duration, fn func() bool) bool {
	return BoolCtx(ctx, false, timeout, interval, fn)
}

// BoolNoErrorCtx waits for fn() to return the specified expected result (either true
// or false) without error while context (with timeout if timeout > 0) is not done
// checking at each interval.
//
//	returns:
//	   false, err: on first fn() with non nil error
//	   true, nil: on first fn() that returns true, nil
//	   false, nil: when context is done
func BoolNoErrorCtx(ctx context.Context, expected bool, timeout, interval time.Duration, fn func() (bool, error)) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		select {
		case <-ctx.Done():
			return false, nil
		default:
			if v, err := fn(); err != nil {
				return false, err
			} else if v == expected {
				return true, nil
			}
			time.Sleep(interval)
		}
	}
}

// TrueNoErrorCtx is BoolNoErrorCtx with expected value true
func TrueNoErrorCtx(ctx context.Context, timeout, interval time.Duration, fn func() (bool, error)) (bool, error) {
	return BoolNoErrorCtx(ctx, true, timeout, interval, fn)
}

// FalseNoErrorCtx is BoolNoErrorCtx with expected value false
func FalseNoErrorCtx(ctx context.Context, timeout, interval time.Duration, fn func() (bool, error)) (bool, error) {
	return BoolNoErrorCtx(ctx, false, timeout, interval, fn)
}
