package capabilities

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) {
	td := t.TempDir()
	SetCacheFile(filepath.Join(td, "capabilities-test.json"))
	scanners = nil
	caps = nil
}

func TestLoad(t *testing.T) {
	t.Run("return ErrorNeedScan when not yet scanned", func(t *testing.T) {
		setup(t)
		_, err := Load()
		assert.Equal(t, ErrorNeedScan, err)
	})
	t.Run("return ErrorNeedScan when current capabilities is corrupt", func(t *testing.T) {
		setup(t)
		assert.Nil(t, os.WriteFile(cacheFile, []byte{}, 0666))
		_, err := Load()
		assert.Equal(t, ErrorNeedScan, err)

		t.Run("can use Has", func(t *testing.T) {
			assert.False(t, Has(""))
			assert.False(t, Has("foo"))
		})
	})
	cases := []struct {
		name        string
		data        []byte
		expectedCap L
	}{
		{"when 2 caps", []byte(`["c1","c2"]`), []string{"c1", "c2"}},
		{"when no caps", []byte(`[]`), []string{}},
	}
	for _, tc := range cases {
		t.Run("succeed and has expected cap "+tc.name, func(t *testing.T) {
			setup(t)
			assert.Nil(t, os.WriteFile(cacheFile, tc.data, 0666))
			loadCaps, err := Load()
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedCap, loadCaps)
		})
	}
}

func TestHas(t *testing.T) {
	cases := []struct {
		name        string
		data        []byte
		expectedCap []string
	}{
		{"when 2 caps", []byte(`["c1","c2"]`), []string{"c1", "c2"}},
		{"when 1 cap", []byte(`["c1"]`), []string{"c1"}},
		{"when no caps", []byte(`[]`), []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setup(t)
			assert.Nil(t, os.WriteFile(cacheFile, tc.data, 0666))
			for _, c := range tc.expectedCap {
				t.Run("has expected "+c, func(t *testing.T) {
					assert.True(t, Has(c))
				})
			}
			t.Run("has not", func(t *testing.T) {
				assert.False(t, Has("foo"))
			})
		})
	}
	t.Run("can be used even if no capacities not yet scanned", func(t *testing.T) {
		setup(t)
		assert.False(t, Has("foo"))
		assert.False(t, Has("bar"))
	})
}

func TestScan(t *testing.T) {
	t.Run("succeed when no Scanner", func(t *testing.T) {
		setup(t)
		assert.Nil(t, Scan(t.Context()))
		assert.Equalf(t, L{}, caps, "must have empty caps")
	})

	t.Run("return error is not able to update cache", func(t *testing.T) {
		setup(t)
		SetCacheFile("/tmp/does-not-exist/capabilities.json")
		err := Scan(t.Context())
		assert.Error(t, err, os.ErrNotExist)
	})

	t.Run("succeed even if some Scanner has errors", func(t *testing.T) {
		setup(t)

		Register(func(context.Context) ([]string, error) { return []string{"c", "b"}, nil })
		Register(func(context.Context) ([]string, error) { return []string{}, nil })
		Register(func(context.Context) ([]string, error) { return []string{}, errors.New("") })
		Register(func(context.Context) ([]string, error) { return []string{"not"}, errors.New("") })
		Register(func(context.Context) ([]string, error) { return []string{"a"}, nil })
		assert.Nil(t, Scan(t.Context()))

		t.Run("has updated itself", func(t *testing.T) {
			assert.True(t, Has("a"))
			assert.True(t, Has("b"))
			assert.True(t, Has("c"))
			assert.Falsef(t, Has("not"), "failed Scanner cap must be ignored")
			assert.Equalf(t, L{"a", "b", "c"}, caps, "must have succeed scanners")
		})
		t.Run("make scanned capabilities persistent", func(t *testing.T) {
			loadedCaps, err := Load()
			assert.Nil(t, err)
			assert.Equal(t, caps, loadedCaps)
		})
	})
}
