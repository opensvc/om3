package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// collector.timeout is used within its documented bounds, its default when
// not set.
func TestBoundTimeout(t *testing.T) {
	assert.Equal(t, DefaultTimeout, BoundTimeout(0))
	assert.Equal(t, time.Second, BoundTimeout(100*time.Millisecond))
	assert.Equal(t, 8*time.Second, BoundTimeout(8*time.Second))
	assert.Equal(t, 20*time.Second, BoundTimeout(time.Minute))
}
