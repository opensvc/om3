package test_conf_helper

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func InstallSvcFile(t *testing.T, sourceName, dstFile string) {
	require.NoError(t, os.MkdirAll(filepath.Dir(dstFile), 0700))
	srcFile := filepath.Join("test-fixtures", sourceName)
	data, err := ioutil.ReadFile(srcFile)
	require.NoError(t, err)
	err = ioutil.WriteFile(dstFile, data, 0644)
	require.NoError(t, err)
}
