package sizeconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	validStrings = map[string]int64{
		"0":     int64(0),
		"1":     int64(1),
		"1000":  int64(1000),
		"100m":  int64(100 * MiB),
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

func TestFromSize(t *testing.T) {
	t.Run("valid sizes", func(t *testing.T) {
		for s, expected := range validStrings {
			result, err := FromSize(s)
			assert.Nilf(t, err, s)
			assert.Equalf(t, expected, result, "FromSize('%v') -> %v", s, result)
		}
	})
	t.Run("invalid sizes", func(t *testing.T) {
		for _, s := range invalidStrings {
			v, err := FromSize(s)
			assert.NotNilf(t, err, "FromSize('%v') -> %v", s, v)
		}
	})
}
