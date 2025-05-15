package executable

import "os"

var (
	ExecutableKey = "OSVC_EXECUTABLE"
)

// The go test codepaths can set a non-test "om" build to prevent fork bomb.
func Path() (string, error) {
	if s := os.Getenv(ExecutableKey); s != "" {
		return s, nil
	}
	return os.Executable()
}

// Set presets the cached executable path.
func Set(p string) {
	os.Setenv(ExecutableKey, p)
}

func Unset() {
	os.Unsetenv(ExecutableKey)
}
