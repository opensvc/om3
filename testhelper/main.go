package testhelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/util/file"
)

func InstallFile(t *testing.T, srcFile, dstFile string) {
	require.NoError(t, os.MkdirAll(filepath.Dir(dstFile), os.ModePerm))
	require.NoError(t, file.Copy(srcFile, dstFile))
}
