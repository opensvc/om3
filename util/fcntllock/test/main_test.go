package fcntllock_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/test_helper"
	"opensvc.com/opensvc/util/fcntllock"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	t.Run("lockfile is created", func(t *testing.T) {
		lockfile, tfCleanup := test_helper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)
		ctx := context.Background()
		err := l.LockContext(ctx, 10*time.Millisecond)
		assert.Equal(t, nil, err)
		_, err = os.Stat(lockfile)
		assert.Nil(t, err)
	})

	t.Run("lock fail if lock dir doesn't exists", func(t *testing.T) {
		lockDir, cleanup := test_helper.Tempdir(t)
		defer cleanup()
		l := fcntllock.New(filepath.Join(lockDir, "dir", "lck"))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := l.LockContext(ctx, 5*time.Millisecond)
		assert.NotNil(t, err)
		assert.Equal(t, "context deadline exceeded", err.Error())
	})
}

func TestUnLock(t *testing.T) {
	t.Run("Ensure unlock (fcntl lock) succeed even if file is not locked", func(t *testing.T) {
		lockfile, tfCleanup := test_helper.TempFile(t)
		defer tfCleanup()
		l := fcntllock.New(lockfile)

		err := l.UnLock()
		assert.Equal(t, nil, err)
	})
}
