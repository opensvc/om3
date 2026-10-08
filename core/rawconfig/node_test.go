package rawconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/render/palette"
)

func TestLoadAppliesTheColorsOfTheEnvFile(t *testing.T) {
	t.Setenv("OSVC_COLORS", "")
	defer Load(map[string]string{})

	// The process environment wins over the env file, so it is left unset
	// for the env file value to apply.
	t.Run("from the env file", func(t *testing.T) {
		unsetenv(t, "OSVC_COLORS")
		Load(map[string]string{"OSVC_COLORS": "error=blue:warning=magenta"})
		p := palette.DefaultPalette()
		p.Error = "blue"
		p.Warning = "magenta"
		assert.Equal(t, *palette.New(*p), *Color)
	})

	t.Run("from the process environment", func(t *testing.T) {
		t.Setenv("OSVC_COLORS", "error=green")
		Load(map[string]string{"OSVC_COLORS": "error=blue"})
		p := palette.DefaultPalette()
		p.Error = "green"
		assert.Equal(t, *palette.New(*p), *Color)
	})
}

func TestLoadSetsTheBoardLetters(t *testing.T) {
	defer Load(map[string]string{})
	cases := []struct {
		name    string
		env     map[string]string
		process string
		want    bool
	}{
		{name: "true by default", env: map[string]string{}, want: true},
		{name: "false from the env file", env: map[string]string{"OSVC_BOARD_LETTERS": "false"}, want: false},
		{name: "a value not a boolean keeps the default", env: map[string]string{"OSVC_BOARD_LETTERS": "nope"}, want: true},
		{name: "the process environment wins", env: map[string]string{"OSVC_BOARD_LETTERS": "true"}, process: "0", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.process != "" {
				t.Setenv("OSVC_BOARD_LETTERS", c.process)
			} else {
				unsetenv(t, "OSVC_BOARD_LETTERS")
			}
			Load(c.env)
			assert.Equal(t, c.want, BoardLetters)
		})
	}
}

// unsetenv unsets the environment variable for the test, and sets it back
// as it was after it.
func unsetenv(t *testing.T, name string) {
	t.Helper()
	prev, ok := os.LookupEnv(name)
	_ = os.Unsetenv(name)
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(name, prev)
		}
	})
}
