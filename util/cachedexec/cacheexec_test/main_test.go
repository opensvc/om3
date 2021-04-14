// Package cachedexec_test provides blackbox tests on cachedexec
package cachedexec_test

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"opensvc.com/opensvc/test_helper"
	"os/exec"
	"testing"
	"time"

	"opensvc.com/opensvc/util/cachedexec"
)

func TestOutput(t *testing.T) {
	tf, cleanup := test_helper.TempFile(t)
	defer cleanup()

	getRealCommandOutput := func() ([]byte, error) {
		value := []byte(time.Now().Format("15:04:05.000000000"))
		return value, ioutil.WriteFile(tf, value, 0644)
	}

	getNewCommand := func() *cachedexec.T {
		return cachedexec.New(exec.Command("/bin/cat", tf))
	}

	defer func(t *testing.T) {
		err := getNewCommand().Clear()
		assert.Nil(t, err)
	}(t)

	t.Run("when cache cleared, return value from command output", func(t *testing.T) {
		command := getNewCommand()
		assert.Nil(t, command.Clear())
		expectedOutput, err := getRealCommandOutput()
		assert.Nil(t, err)
		for i := 0; i < 3; i++ {
			out, err := command.Output()
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("return value from cache", func(t *testing.T) {
		command := getNewCommand()
		assert.Nil(t, command.Clear())
		expectedOutput, err := getRealCommandOutput()
		assert.Nil(t, err)

		// feed cache
		_, err = command.Output()
		assert.Nil(t, err)

		// reset real command output
		_, err = getRealCommandOutput()
		assert.Nil(t, err)

		output, err := command.Output()
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output)
	})

	t.Run("ensure Clear() cleanup cache", func(t *testing.T) {
		_, err := getRealCommandOutput()
		assert.Nil(t, err)
		command := getNewCommand()
		initialData, err := command.Output()
		assert.Nil(t, err)
		assert.NotNil(t, initialData)

		newCommand := getNewCommand()
		assert.Nil(t, newCommand.Clear())
		newExpected, err := getRealCommandOutput()
		assert.Nil(t, err)

		output, err := newCommand.Output()
		assert.Nil(t, err)
		assert.Equal(t, newExpected, output)
	})
}
