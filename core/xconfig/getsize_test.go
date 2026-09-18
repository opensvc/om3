package xconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/util/key"
)

// plainReferrer declares keywords with no converter, the way the loop, lv and
// rados disks declare the size they are given.
type plainReferrer struct {
	config *T
}

func (t *plainReferrer) KeywordLookup(k key.T, _ string) *keywords.Keyword {
	if k.Section == "disk#0" {
		return &keywords.Keyword{Option: k.Option, Section: "disk#0"}
	}
	return nil
}

func (t *plainReferrer) IsVolatile() bool                     { return true }
func (t *plainReferrer) Config() *T                           { return t.config }
func (t *plainReferrer) ConfigData() any                      { return nil }
func (t *plainReferrer) Dereference(s string) (string, error) { return s, nil }
func (t *plainReferrer) Nodes() ([]string, error)             { return []string{"n1"}, nil }
func (t *plainReferrer) DRPNodes() ([]string, error)          { return nil, nil }

func newPlainConfig(t *testing.T, body string) *T {
	t.Helper()
	cfg, err := NewObject("", []byte(body))
	require.NoError(t, err)
	cfg.Referrer = &plainReferrer{config: cfg}
	return cfg
}

// A keyword spelling a size and declaring no converter evaluates to a string.
// Asking it for its size must answer rather than take the process down.
//
//	panic: interface conversion: interface {} is string, not *int64
//	xconfig.(*T).GetSizeStrict
func TestGetSizeOfAKeywordWithNoConverter(t *testing.T) {
	cfg := newPlainConfig(t, "[disk#0]\nsize = 200m\nzero = 0\nempty =\nnonsense = banana\n")

	assert.NotPanics(t, func() { _ = cfg.GetSize(key.New("disk#0", "size")) })

	size, err := cfg.GetSizeStrict(key.New("disk#0", "size"))
	assert.NoError(t, err)
	require.NotNil(t, size)
	assert.Equal(t, int64(209715200), *size, "read the way the configuration spells it")

	// A size of zero is a size, and says the resource holds nothing rather
	// than that it holds nothing worth reading.
	size, err = cfg.GetSizeStrict(key.New("disk#0", "zero"))
	assert.NoError(t, err)
	require.NotNil(t, size)
	assert.Equal(t, int64(0), *size)

	// A keyword set to nothing has no size.
	size, err = cfg.GetSizeStrict(key.New("disk#0", "empty"))
	assert.NoError(t, err)
	assert.Nil(t, size)

	// A value that is not a size is an error, not a panic and not a zero.
	_, err = cfg.GetSizeStrict(key.New("disk#0", "nonsense"))
	assert.ErrorIs(t, err, ErrType)
}

// The same shape, for the two other getters that asserted what the converter
// returned instead of reading it.
func TestGetStringsAndGetSetOfAKeywordWithNoConverter(t *testing.T) {
	cfg := newPlainConfig(t, "[disk#0]\ndevs = /dev/a /dev/b\nempty =\n")

	assert.NotPanics(t, func() { _ = cfg.GetStrings(key.New("disk#0", "devs")) })
	l, err := cfg.GetStringsStrict(key.New("disk#0", "devs"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"/dev/a", "/dev/b"}, l)

	l, err = cfg.GetStringsStrict(key.New("disk#0", "empty"))
	assert.NoError(t, err)
	assert.Empty(t, l)

	assert.NotPanics(t, func() { _ = cfg.GetSet(key.New("disk#0", "devs")) })
	s, err := cfg.GetSetStrict(key.New("disk#0", "devs"))
	assert.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, 2, s.Len())
	assert.True(t, s.Has("/dev/a"))
	assert.True(t, s.Has("/dev/b"))
}
