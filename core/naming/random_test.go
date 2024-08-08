package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandom(t *testing.T) {
	for i := 0; i < 10; i += 1 {
		s := Random()
		t.Logf("random name: %s", s)
		assert.LessOrEqual(t, len(s), 16, "random name %s should have less than 17 chars", s)
	}
}
