package waitfor

import (
	"context"
	"testing"
	"time"
)

// A wait ends with its timeout, not up to an interval after: the stop of a
// container kept time for its forced stop after the wait, which the sleep of
// an interval past the timeout used up.
func TestWaitEndsWithItsTimeout(t *testing.T) {
	never := func() bool { return false }
	start := time.Now()
	if TrueCtx(context.Background(), 100*time.Millisecond, 2*time.Second, never) {
		t.Fatal("TrueCtx: true from a condition never met")
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("TrueCtx ended %s after its start, its timeout 100ms", d)
	}
	start = time.Now()
	if v, err := TrueNoErrorCtx(context.Background(), 100*time.Millisecond, 2*time.Second, func() (bool, error) { return false, nil }); v || err != nil {
		t.Fatalf("TrueNoErrorCtx: %v %v", v, err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("TrueNoErrorCtx ended %s after its start, its timeout 100ms", d)
	}
}
