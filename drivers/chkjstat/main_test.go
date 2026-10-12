package chkjstat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	out := "  S0     S1     E      O      M     CCS    YGC     YGCT    FGC    FGCT     GCT\n" +
		"  0,00  99,80  45,12  12,40  97,33     -     12    0,123     0    0,000    0,123\n"
	assert.Equal(t, []metric{
		{"S0", 0}, {"S1", 100}, {"E", 45}, {"O", 12}, {"M", 97},
		{"YGC", 12}, {"YGCT", 0}, {"FGC", 0}, {"FGCT", 0}, {"GCT", 0},
	}, parse([]byte(out)))
}

func TestFromEnviron(t *testing.T) {
	j, ok := fromEnviron("12", []byte("PATH=/bin\x00OPENSVC_RID=app#1\x00OPENSVC_SVCPATH=ns1/svc/web\x00"))
	assert.True(t, ok)
	assert.Equal(t, jvm{pid: "12", instance: "app#1", path: "ns1/svc/web"}, j)
	_, ok = fromEnviron("13", []byte("PATH=/bin\x00"))
	assert.False(t, ok, "a java the agent did not start is not reported")
}

// jstat runs only when it is a file of its owner, or of root, that no one
// else may have replaced.
func TestIsTrustedBinary(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte{}, mode))
		require.NoError(t, os.Chmod(p, mode))
		return p
	}
	uid := uint32(os.Getuid())
	assert.True(t, isTrustedBinary(write("ok", 0o755), uid))
	assert.False(t, isTrustedBinary(write("groupw", 0o775), uid))
	assert.False(t, isTrustedBinary(write("otherw", 0o757), uid))
	assert.False(t, isTrustedBinary(filepath.Join(dir, "missing"), uid))
	assert.False(t, isTrustedBinary(dir, uid), "a directory")
	if uid == 0 {
		assert.True(t, isTrustedBinary(write("root", 0o755), 1000), "root's file for another user")
	} else {
		assert.False(t, isTrustedBinary(write("other", 0o755), uid+1), "a file of another user")
	}
}
