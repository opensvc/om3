package xconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/converters"
)

// A keyword evaluated to an empty value its converter refuses, as an integer
// unset with no default, says ErrNoValue; a value refused, or an empty value
// its converter accepts, does not.
func TestConvertNoValue(t *testing.T) {
	cfg := &T{}
	kwInt := &keywords.Keyword{Option: "n", Converter: converters.Int}
	_, err := cfg.convert("", kwInt)
	assert.ErrorIs(t, err, ErrNoValue)
	_, err = cfg.convert("x", kwInt)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoValue)
	v, err := cfg.convert("", &keywords.Keyword{Option: "b", Converter: converters.Bool})
	assert.NoError(t, err)
	assert.Equal(t, false, v)
}
