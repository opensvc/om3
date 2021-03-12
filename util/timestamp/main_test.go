package timestamp

import (
	"testing"
)

func TestTimestamp(t *testing.T) {
	ts := New()
	s := ts.String()
	t.Logf("%s", s)
}
