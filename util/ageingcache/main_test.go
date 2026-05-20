package ageingcache

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
	defer rawconfig.ReloadForTest(td)()

	getRealCommandOutput := func() ([]byte, error) {
		value := []byte(time.Now().Format("15:04:05.000000000"))
		return value, os.WriteFile(tf, value, 0644)
	}

	t.Run("return value from command output", func(t *testing.T) {
		for _, sig := range []string{"a", "b", "c"} {
			expectedOutput, err := getRealCommandOutput()
			assert.Nil(t, err)
			out, err := Output(exec.Command("cat", tf), sig, 1*time.Hour)
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("return value from cache", func(t *testing.T) {
		expectedOutput, err := getRealCommandOutput()
		assert.Nil(t, err)

		// feed cache
		_, err = Output(exec.Command("cat", tf), "cat-on-tf", 1*time.Hour)
		assert.Nil(t, err)

		// reset real command output
		_, err = getRealCommandOutput()
		assert.Nil(t, err)

		for range []int{1, 2, 3} {
			out, err := Output(exec.Command("cat", tf), "cat-on-tf", 1*time.Hour)
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("expire cache when older than maxAge", func(t *testing.T) {
		// feed cache
		expectedOutput1, err := getRealCommandOutput()
		assert.Nil(t, err)
		_, err = Output(exec.Command("cat", tf), "expire-test", 1*time.Second)
		assert.Nil(t, err)

		// wait for cache to expire
		time.Sleep(2 * time.Second)

		// reset real command output
		expectedOutput2, err := getRealCommandOutput()
		assert.Nil(t, err)

		// should get fresh output since cache expired
		out, err := Output(exec.Command("cat", tf), "expire-test", 1*time.Second)
		assert.Nil(t, err)
		assert.Equalf(t, expectedOutput2, out, "%q vs %q\n", expectedOutput2, out)
		assert.NotEqual(t, expectedOutput1, out)
	})

	t.Run("ensure Clear() cleanup cache", func(t *testing.T) {
		for range []int{1, 2, 3} {
			expected, err := getRealCommandOutput()
			assert.Nil(t, err)
			assert.Nil(t, Clear("clear-test"))
			out, err := Output(exec.Command("cat", tf), "clear-test", 1*time.Hour)
			assert.Nil(t, err)
			assert.Equalf(t, expected, out, "%q vs %q\n", expected, out)
		}
	})
}
