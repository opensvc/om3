package fcache

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

func TestOutput(t *testing.T) {
	td := t.TempDir()
	tf, cleanup := testhelper.TempFile(t, td)
	defer cleanup()
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": td})
	defer rawconfig.Load(map[string]string{})

	getRealCommandOutput := func() ([]byte, error) {
		value := []byte(time.Now().Format("15:04:05.000000000"))
		return value, os.WriteFile(tf, value, 0644)
	}

	t.Run("return value from command output", func(t *testing.T) {
		for _, sig := range []string{"a", "b", "c"} {
			expectedOutput, err := getRealCommandOutput()
			assert.Nil(t, err)
			out, err := Output(exec.Command("cat", tf), sig)
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("return value from cache", func(t *testing.T) {
		expectedOutput, err := getRealCommandOutput()
		assert.Nil(t, err)

		// feed cache
		_, err = Output(exec.Command("cat", tf), "cat-on-tf")
		assert.Nil(t, err)

		// reset real command output
		_, err = getRealCommandOutput()
		assert.Nil(t, err)

		for range []int{1, 2, 3} {
			out, err := Output(exec.Command("cat", tf), "cat-on-tf")
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("ensure Clear() cleanup cache", func(t *testing.T) {
		for range []int{1, 2, 3} {
			expected, err := getRealCommandOutput()
			assert.Nil(t, PurgeCache())
			out, err := Output(exec.Command("cat", tf), "cat-on-tf")
			assert.Nil(t, err)
			assert.Equalf(t, expected, out, "%q vs %q\n", expected, out)
		}
	})
}
