package sysproc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPidFromPort(t *testing.T) {
	testCases := map[string]struct {
		testDir     string
		expectError bool
		expectedPid int
	}{
		"the listening tcp and the socket are in the same pid": {
			testDir:     "./testdata/procDir_with_listening_tcp_in_the_same_pid",
			expectError: false,
			expectedPid: 5,
		},

		"the listening tcp and the socket are not in the same pid": {
			testDir:     "./testdata/procDir_with_listening_tcp_in_not_the_same_pid",
			expectError: false,
			expectedPid: 5,
		},

		"the listening tcp and the socket are not in the same pid but using tcp6": {
			testDir:     "./testdata/procDir_with_listening_tcp6_in_the_same_pid",
			expectError: false,
			expectedPid: 5,
		},

		"no listening tcp": {
			testDir:     "./testdata/procDir_with_no_listening",
			expectError: true,
			expectedPid: -1,
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			oriOsReadLink := osReadLink
			defer func() { osReadLink = oriOsReadLink }()

			oriOsReadFile := osReadFile
			defer func() { osReadFile = oriOsReadFile }()

			oriOsReadDir := osReadDir
			defer func() { osReadDir = oriOsReadDir }()

			osReadLink = func(p string) (string, error) {
				b, err := os.ReadFile(filepath.Join(c.testDir, p))
				return string(b), err
			}

			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(filepath.Join(c.testDir, name))
			}

			osReadDir = func(name string) ([]os.DirEntry, error) {
				return os.ReadDir(filepath.Join(c.testDir, name))
			}

			oriGetParentPid := fGetParentPid
			defer func() {
				getParentPid = oriGetParentPid
			}()

			getParentPid = func(pid int) (int, error) {
				return pid, nil
			}

			pid, err := GetPidFromPort(22)

			if c.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, c.expectedPid, pid)
		})
	}
}

func TestGetParentPid(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	oriOsReadLink := osReadLink
	defer func() { osReadLink = oriOsReadLink }()

	testCases := map[string]struct {
		testDirPath  string
		oriPid       int
		expectedPpid int
	}{
		"with pid that is the ppid": {
			testDirPath:  "./testdata/procDir_getParentPid",
			oriPid:       2,
			expectedPpid: 2,
		},

		"with pid that is not the ppid": {
			testDirPath:  "./testdata/procDir_getParentPid",
			oriPid:       4,
			expectedPpid: 2,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(filepath.Join(c.testDirPath, name))
			}

			osReadLink = func(p string) (string, error) {
				b, err := os.ReadFile(filepath.Join(c.testDirPath, p))
				return string(b), err
			}

			ppid, err := fGetParentPid(c.oriPid)
			require.NoError(t, err)
			require.Equal(t, c.expectedPpid, ppid)
		})
	}
}
