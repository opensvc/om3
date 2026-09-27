package converters

import (
	"os"
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
				result, err := Duration.Convert(s)
				require.NoError(t, err)
				assert.Equal(t, expected, *result.(*time.Duration))
			})
		}
	})

	t.Run("empty String returns nil", func(t *testing.T) {
		result, err := Duration.Convert("")
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid duration expression returns (nil, error)", func(t *testing.T) {
		for _, s := range invalidStrings {
			t.Run(s, func(t *testing.T) {
				_, err := Duration.Convert(s)
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
				result, err := Size.Convert(s)
				assert.Nilf(t, err, s)
				resultInt64 := *result.(*int64)
				assert.Equalf(t, expected, resultInt64, "ToSize('%v') -> %v", s, resultInt64)
			})
		}
	})

	t.Run("empty String return nil", func(t *testing.T) {
		result, err := Size.Convert("")
		assert.Nil(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid size return (nil, error)", func(t *testing.T) {
		for _, s := range invalidStrings {
			t.Run(s, func(t *testing.T) {
				result, err := Size.Convert(s)
				assert.NotNilf(t, err, "FromSize('%v') error is not nil", s)
				assert.Nil(t, result, "FromSize('%v') return pointer is not nil", s)
			})
		}
	})
}

func TestBooleanConvert(t *testing.T) {
	var (
		validStrings = map[string]bool{
			"true":  true,
			"TRUE":  true,
			"yes":   true,
			"Yes":   true,
			"y":     true,
			"t":     true,
			"T":     true,
			"1":     true,
			"":      false,
			"false": false,
			"False": false,
			"no":    false,
			"NO":    false,
			"n":     false,
			"f":     false,
			"N":     false,
			"0":     false,
			"0.0":   false,
			"none":  false,
			"[]":    false,
			"{}":    false,
		}

		invalidStrings = []string{
			"maybe",
			"2",
			"oui",
			"non",
			"o",
			"n0",
		}
	)

	t.Run("valid boolean expression returns expected values", func(t *testing.T) {
		for s, expected := range validStrings {
			t.Run(s, func(t *testing.T) {
				result, err := Bool.Convert(s)
				require.NoError(t, err)
				assert.Equal(t, expected, result.(bool))
			})
		}
	})

	t.Run("invalid boolean expression returns (nil, error)", func(t *testing.T) {
		for _, s := range invalidStrings {
			t.Run(s, func(t *testing.T) {
				_, err := Bool.Convert(s)
				assert.Error(t, err)
			})
		}
	})
}

// The leading digit of a mode is read as chmod reads it, and as v2 did.
func TestFileModeConvertReadsTheSpecialBitsAsChmod(t *testing.T) {
	for s, want := range map[string]os.FileMode{
		"644":  0o644,
		"0644": 0o644,
		"1777": os.ModeSticky | 0o777,
		"2775": os.ModeSetgid | 0o775,
		"4755": os.ModeSetuid | 0o755,
		"6755": os.ModeSetuid | os.ModeSetgid | 0o755,
		"7777": os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0o777,
	} {
		v, err := FileMode.Convert(s)
		require.NoError(t, err, s)
		assert.Equal(t, want, *v.(*os.FileMode), s)
	}
	for _, s := range []string{"", "8", "12345", "0x44", "abc"} {
		v, err := FileMode.Convert(s)
		if s == "" {
			assert.NoError(t, err)
			assert.Nil(t, v.(*os.FileMode))
			continue
		}
		assert.Error(t, err, s)
	}
}
