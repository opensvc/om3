// Package homedir expands the ~ of a path to the home directory of the user.
package homedir

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
)

// Expand returns path with a leading "~" replaced by the home directory of
// the user, as os.UserHomeDir says it: $HOME on unix and macOS, else the
// user database, as for a service or a sudo run with no $HOME. A path not
// starting with "~" is returned as is, and a "~user" one is refused, as the
// home of another user is not looked up.
func Expand(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if len(path) > 1 && !os.IsPathSeparator(path[1]) {
		return "", errors.New("cannot expand a user-specific home directory")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		u, uErr := user.Current()
		if uErr != nil || u.HomeDir == "" {
			return "", errors.Join(err, uErr)
		}
		home = u.HomeDir
	}
	return filepath.Join(home, path[1:]), nil
}
