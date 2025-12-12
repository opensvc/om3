package bootid_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/bootid"
)

func Test_Get(t *testing.T) {
	s := bootid.Get()
	switch runtime.GOOS {
	case "linux":
		require.NotEmpty(t, s, "unexpected empty node boot id")
	default:
		require.Equal(t, "", s)
	}
}

func Test_Scan(t *testing.T) {
	s, err := bootid.Scan()
	switch runtime.GOOS {
	case "linux":
		require.NoError(t, err, "unexpected scan error")
		require.NotEmpty(t, s, "unexpected empty node boot id")
	default:
		require.Error(t, err)
	}
}

func Test_Set(t *testing.T) {
	defer func() {
		// force rescan after test
		bootid.Scan()
	}()
	bootid.Set("abcd")
	require.Equal(t, "abcd", bootid.Get(), "Get value should return the value Set")
}
