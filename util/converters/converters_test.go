package converters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurationConvert(t *testing.T) {
	var (
		validStrings = map[string]time.Duration{
			"0":    0 * time.Second,
			"1":    1 * time.Second,
			"1s":   1 * time.Second,
			"1m1s": 61 * time.Second,
			"1d":   24 * time.Hour,
			"1w":   7 * 24 * time.Hour,
			"1y":   365 * 24 * time.Hour,
		}
		invalidStrings = []string{
			"1p",
		}
	)

	t.Run("valid duration expression returns expected values", func(t *testing.T) {
		for s, expected := range validStrings {
			t.Run(s, func(t *testing.T) {
				result, err := Lookup("duration").Convert(s)
				require.NoError(t, err)
				assert.Equal(t, expected, *result.(*time.Duration))
			})
		}
	})

	t.Run("empty String returns nil", func(t *testing.T) {
		result, err := Lookup("duration").Convert("")
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid duration expression returns (nil, error)", func(t *testing.T) {
		for _, s := range invalidStrings {
			t.Run(s, func(t *testing.T) {
				_, err := Lookup("duration").Convert(s)
				assert.Error(t, err)
			})
		}
	})
}

func TestSizeConvert(t *testing.T) {
	var (
		validStrings = map[string]int64{
			"0":     int64(0),
			"1":     int64(1),
			"1000":  int64(1000),
			"1KB":   int64(1000),
			"1,3KB": int64(1300),
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
			"8EiB",
			"badValue",
		}
	)

	t.Run("Valid String return expected values", func(t *testing.T) {
		for s, expected := range validStrings {
			t.Run(s, func(t *testing.T) {
				result, err := Lookup("size").Convert(s)
				assert.Nilf(t, err, s)
				resultInt64 := *result.(*int64)
				assert.Equalf(t, expected, resultInt64, "ToSize('%v') -> %v", s, resultInt64)
			})
		}
	})

	t.Run("empty String return nil", func(t *testing.T) {
		result, err := Lookup("size").Convert("")
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid size return (nil, error)", func(t *testing.T) {
		for _, s := range invalidStrings {
			t.Run(s, func(t *testing.T) {
				result, err := Lookup("size").Convert(s)
				assert.NotNilf(t, err, "FromSize('%v') error is not nil", s)
				assert.Nil(t, result, "FromSize('%v') return pointer is not nil", s)
			})
		}
	})
}
