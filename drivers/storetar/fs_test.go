package storetar

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
)

// newSFTPStore returns a store at dir of an in-process sftp server: the
// store files are written and read through the sftp protocol, the ssh
// connection and its keys left out.
func newSFTPStore(t *testing.T, dir string) *T {
	t.Helper()
	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	server, err := sftp.NewServer(struct {
		io.Reader
		io.WriteCloser
	}{serverR, serverW})
	require.NoError(t, err)
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientR, clientW)
	require.NoError(t, err)
	s := newStore(t, "sftp://backup@sftp.example"+dir)
	s.fsys = &sftpFS{client: client}
	t.Cleanup(func() {
		// The client waits for the server to close its end.
		_ = server.Close()
		_ = s.Close()
	})
	return s
}

// A store on an sftp server backs up, lists, completes, restores and deletes
// runs as one in a directory of the node does.
func TestSFTPStore(t *testing.T) {
	ctx := context.Background()
	storeDir := t.TempDir()
	s := newSFTPStore(t, storeDir)
	assert.False(t, s.IsLocal())

	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "a", 0640)
	writeFile(t, filepath.Join(src, "sub", "b.txt"), "b", 0600)
	run := newRun("run1", time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC))
	require.NoError(t, s.Backup(ctx, run, "fs#1", src, nil))
	require.NoError(t, s.BackupStream(ctx, run, backup.MemberConfig, strings.NewReader("[DEFAULT]\n")))
	require.NoError(t, s.Complete(ctx, run))
	assert.FileExists(t, filepath.Join(storeDir, "c1", "oid", "run1", "fs#1.tar.gz"), "written on the server")
	assert.FileExists(t, filepath.Join(storeDir, "c1", "oid", markerName), "the path of the object recorded")

	runs, err := s.Runs(ctx, backup.RunFilter{Path: "ns1/svc/svc1"})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, []string{"fs#1", "config"}, runs[0].MemberNames())
	assert.False(t, runs[0].CompletedAt.IsZero(), "the manifest rewritten in place")

	dst := t.TempDir()
	require.NoError(t, s.Restore(ctx, runs[0], "fs#1", dst))
	b, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "b", string(b))
	require.NoError(t, s.Check(ctx, backup.RunFilter{}))

	require.NoError(t, s.Delete(ctx, runs))
	runs, err = s.Runs(ctx, backup.RunFilter{Incomplete: true})
	require.NoError(t, err)
	assert.Empty(t, runs)
}

// The url and the keys of an sftp store are checked before any connection.
func TestSFTPStoreRefusals(t *testing.T) {
	_, _, err := newStore(t, "sftp://sftp.example/srv").location()
	assert.ErrorContains(t, err, "names no user")
	_, _, err = newStore(t, "sftp://backup@sftp.example/srv").location()
	assert.ErrorContains(t, err, "no host_key")
	_, _, err = newStore(t, "sftp://backup@sftp.example").location()
	assert.ErrorContains(t, err, "no absolute path")
	assert.True(t, newStore(t, "/srv/backups").IsLocal())
}
