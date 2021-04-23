package capabilities

import (
	"errors"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

type (
	scannerMock struct {
		caps []string
		err  error
	}
)

func (s scannerMock) Scan() ([]string, error) {
	return s.caps, s.err
}

func TestInit(t *testing.T) {
	t.Run("return ErrorNeedScan when not yet scanned", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		cleanup()
		assert.Equal(t, ErrorNeedScan, New(tf).Init())
	})
	t.Run("return ErrorNeedReScan when current capabilities is corrupt", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		assert.Nil(t, ioutil.WriteFile(tf, []byte{}, 0666))
		assert.Equal(t, ErrorNeedScan, New(tf).Init())
	})
	cases := []struct {
		name        string
		data        []byte
		expectedCap []string
	}{
		{"when 2 caps", []byte(`["c1","c2"]`), []string{"c1", "c2"}},
		{"when no caps", []byte(`[]`), []string{}},
	}
	for _, tc := range cases {
		t.Run("succeed "+tc.name, func(t *testing.T) {
			tf, cleanup := testhelper.TempFile(t)
			defer cleanup()
			assert.Nil(t, ioutil.WriteFile(tf, tc.data, 0666))
			assert.Nil(t, New(tf).Init())
		})
		t.Run("has expected cap "+tc.name, func(t *testing.T) {
			tf, cleanup := testhelper.TempFile(t)
			defer cleanup()
			assert.Nil(t, ioutil.WriteFile(tf, tc.data, 0666))
			c := New(tf)
			assert.Nil(t, c.Init())
			assert.Equal(t, tc.expectedCap, c.caps)
		})
	}
}

func TestHas(t *testing.T) {
	c := T{caps: []string{"a", "b"}}
	assert.True(t, c.Has("a"))
	assert.True(t, c.Has("b"))
	assert.False(t, c.Has("not"))
}

func TestScan(t *testing.T) {
	t.Run("when no Scanner", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		c := New(tf)
		assert.Nil(t, c.Scan())
		assert.Equalf(t, []string{}, c.caps, "must have empty caps")
	})

	t.Run("return error is not able to update cache", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		c := New(filepath.Join(tf, "not-possible"))
		err := c.Scan()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("succeed even if some Scanner has errors", func(t *testing.T) {
		tf, cleanup := testhelper.TempFile(t)
		defer cleanup()
		c := New(tf)
		c.Register(scannerMock{caps: []string{"c", "b"}})
		c.Register(scannerMock{})
		c.Register(scannerMock{err: errors.New("")})
		c.Register(scannerMock{caps: []string{"not"}, err: errors.New("")})
		c.Register(scannerMock{caps: []string{"a"}})
		assert.Nil(t, c.Scan())

		t.Run("has updated itself", func(t *testing.T) {
			assert.True(t, c.Has("a"))
			assert.True(t, c.Has("b"))
			assert.True(t, c.Has("c"))
			assert.Falsef(t, c.Has("not"), "failed Scanner cap must be ignored")
			assert.Equalf(t, []string{"a", "b", "c"}, c.caps, "must have succeed scanners")
		})
		t.Run("make scanned capabilities persistent", func(t *testing.T) {
			other := New(tf)
			assert.Nil(t, other.Init())
			assert.Equal(t, c.caps, other.caps)
		})
	})
}
