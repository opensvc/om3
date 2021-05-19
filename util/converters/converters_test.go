package converters

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	validStrings = map[string]int64{
		"0":     int64(0),
		"1":     int64(1),
		"1000":  int64(1000),
		"1KB":   int64(1000),
		"1KiB":  int64(1024),
		"2MiB":  int64(2 * 1024 * 1024),
		"3GiB":  int64(3 * 1024 * 1024 * 1024),
		"3gib":  int64(3 * 1024 * 1024 * 1024),
		"4TiB":  int64(4 * 1024 * 1024 * 1024 * 1024),
		"5PiB":  int64(5 * 1024 * 1024 * 1024 * 1024 * 1024),
		"6EiB":  int64(6 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024),
		"6eib":  int64(6 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024),
		"8EB":   int64(8 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000),
		"8.5EB": int64(8.5 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000),
		"8.5eb": int64(8.5 * 1000 * 1000 * 1000 * 1000 * 1000 * 1000),
	}
	invalidStrings = []string{
		"-1",
		"-1000",
		"-1KB",
		"1,3KB",
		"8EiB",
		"badValue",
	}
)

func TestToSize(t *testing.T) {
	t.Run("Valid String", func(t *testing.T) {
		for s, expected := range validStrings {
			result, err := ToSize(s)
			assert.Nilf(t, err, s)
			assert.Equalf(t, expected, *result, "ToSize('%v') -> %v", s, result)
		}
	})

	t.Run("empty String return nil", func(t *testing.T) {
		result, err := ToSize("")
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid sizes", func(t *testing.T) {
		for _, s := range invalidStrings {
			var pti *int64
			var err error
			pti, err = ToSize(s)
			assert.NotNilf(t, err, "FromSize('%v') error is not nil", s)
			assert.Nil(t, pti, "FromSize('%v') return pointer is not nil", s)
		}
	})
}
