package test_conf_helper

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func InstallSvcFile(t *testing.T, sourceName, dstFile string) {
	require.Nil(t, os.MkdirAll(filepath.Dir(dstFile), 0700))
	srcFile := filepath.Join("test-fixtures", sourceName)
	data, err := ioutil.ReadFile(srcFile)
	require.Nil(t, err)
	err = ioutil.WriteFile(dstFile, data, 0644)
	require.Nil(t, err)
}
