package object

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A key value refused for its size says both sizes and the keyword setting
// the limit, and is still told by the sentinel.
func TestValueTooBigNamesTheLimit(t *testing.T) {
	err := ValueTooBig(2*1024*1024, 1024*1024)
	assert.True(t, errors.Is(err, ErrValueTooBig))
	assert.Equal(t, "key value exceeds the allowed size: 2.00MiB > 1.00MiB (raise node.max_key_size)", err.Error())
}
