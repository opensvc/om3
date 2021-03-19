package timestamp

import (
	"testing"
)

func TestTimestamp(t *testing.T) {
	ts := Now()
	s := ts.String()
	t.Logf("%s", s)
}
