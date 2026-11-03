package nodestats

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

// A node without sysstat has nothing to collect, which the caller tells
// apart from a failure.
func TestCollectWithoutSysstat(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	t.Setenv("PATH", t.TempDir())
	end := time.Now()
	_, err := Collect(context.Background(), Options{Begin: end.Add(-time.Hour), End: end})
	if !errors.Is(err, ErrNoSysstat) {
		t.Fatalf("want ErrNoSysstat, got %v", err)
	}
}
