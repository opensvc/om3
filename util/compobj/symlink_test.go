package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlink(t *testing.T) {
	makeObj := func(testPath string) I {
		obj := NewCompSymlinks().(I)
		rulesList := []string{
			fmt.Sprintf(`{"Symlink":"%s","Target":"target1"}`, filepath.Join(testPath, "testlink1")),
			fmt.Sprintf(`{"Symlink":"%s","Target":"target1"}`, filepath.Join(testPath, "testlink2")),
		}
		for _, rule := range rulesList {
			if err := obj.Add(rule); err != nil {
				require.NoError(t, err)
			}
		}
		return obj
	}

	testPath := t.TempDir()
	obj := makeObj(testPath)
	assert.Equal(t, obj.Check(), ExitNok)

	err := os.Symlink("target1", filepath.Join(testPath, "testlink1"))
	if err != nil {
		require.NoError(t, err)
	}
	err = os.Symlink("WrongTarget", filepath.Join(testPath, "testlink2"))
	if err != nil {
		require.NoError(t, err)
	}
	assert.Equal(t, obj.Check(), ExitNok)
	obj.Fix()
	assert.Equal(t, obj.Check(), ExitNok)

	testPath = t.TempDir()
	obj = makeObj(testPath)
	err = os.Symlink("target1", filepath.Join(testPath, "testlink1"))
	if err != nil {
		require.NoError(t, err)
	}
	obj.Fix()
	assert.Equal(t, obj.Check(), ExitOk)

}
