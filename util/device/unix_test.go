package device

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	fileInfo map[string]struct {
		major uint32
		minor uint32
	}
)

var (
	cases = map[string]fileInfo{
		"darwin": {
			"/dev/null": {3, 2},
		},
		"linux": {
			"/dev/null": {1, 3},
		},
		"solaris": {
			"/dev/null": {118, 2},
		},
	}
)

func TestT_MajorMinor(t *testing.T) {
	for name, info := range cases[runtime.GOOS] {
		t.Run(name, func(t *testing.T) {
			major, minor, err := New(name).MajorMinor()
			if err != nil {
				require.Nil(t, err)
			}
			assert.Equal(t, info.major, major)
			assert.Equal(t, info.minor, minor)
		})
	}
}

func TestT_Major(t *testing.T) {
	for name, info := range cases[runtime.GOOS] {
		t.Run(name, func(t *testing.T) {
			major, err := New(name).Major()
			if err != nil {
				require.Nil(t, err)
			}
			assert.Equal(t, info.major, major)
		})
	}
}

func TestT_Minor(t *testing.T) {
	for name, info := range cases[runtime.GOOS] {
		t.Run(name, func(t *testing.T) {
			minor, err := New(name).Minor()
			if err != nil {
				require.Nil(t, err)
			}
			assert.Equal(t, info.minor, minor)
		})
	}
}
