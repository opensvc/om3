package flock

import (
	"encoding/json"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/test_helper"
	mockFcntlLock "opensvc.com/opensvc/util/mock_fnctllock"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setup(t *testing.T) (prov func(string) locker, lck *mockFcntlLock.MockLocker) {
	ctrl := gomock.NewController(t)
	lck = mockFcntlLock.NewMockLocker(ctrl)
	prov = func(string) locker {
		return lck
	}
	return
}

func TestLock(t *testing.T) {
	t.Run("Ensure write data to lock file when lock succeed", func(t *testing.T) {
		prov, mockLck := setup(t)
		lck := NewCustomLock("foo.lck", prov)
		var b []byte
		mockLck.EXPECT().LockContext(gomock.Any(), 500*time.Millisecond).Return(nil)
		mockLck.EXPECT().
			Write(gomock.AssignableToTypeOf(b)).
			Do(func(arg []byte) {
				b = arg
			}).
			Return(0, nil)

		err := lck.Lock(100*time.Millisecond, "intent1")
		assert.Equal(t, nil, err)

		found := meta{}
		if err := json.Unmarshal(b, &found); err != nil {
			t.Fatalf("expected written data : %+v\n", b)
		}
		assert.Equal(t, "intent1", found.Intent)
	})

	t.Run("Ensure return error if lock fail", func(t *testing.T) {
		prov, mockLck := setup(t)

		mockLck.EXPECT().LockContext(gomock.Any(), gomock.Any()).Return(errors.New("can't lock"))

		err := NewCustomLock("foo.lck", prov).Lock(100*time.Millisecond, "intent1")
		assert.Equal(t, errors.New("can't lock"), err)
	})

	t.Run("can write, seek, read on locked file", func(t *testing.T) {
		lockfile, tfCleanup := test_helper.TempFile(t)
		defer tfCleanup()
		l := New(lockfile)
		err := l.Lock(1*time.Second, "plop")
		assert.Equal(t, nil, err)
		dataToWrite := []byte("{}")
		writeLen, err := l.Write(dataToWrite)
		assert.Nil(t, err)
		assert.Equal(t, 2, writeLen)
		_, err = l.Seek(0, 0)
		assert.Nil(t, err)
		data := make([]byte, 200)
		assert.Nil(t, err)
		readLen, err := l.Read(data)
		assert.Greater(t, readLen, 50)
		_, err = os.Stat(lockfile)
		assert.Nil(t, err)
	})

	t.Run("lock fail if lock dir doesn't exists", func(t *testing.T) {
		lockDir, cleanup := test_helper.Tempdir(t)
		defer cleanup()
		defaultRetryInterval := retryInterval
		defer func() { retryInterval = defaultRetryInterval }()
		retryInterval = 5 * time.Millisecond
		l := New(filepath.Join(lockDir, "dir", "lockfile"))
		err := l.Lock(15*time.Millisecond, "plop")
		assert.NotNil(t, err)
		assert.Equal(t, "lock timeout exceeded", err.Error())
	})
}

func TestUnLock(t *testing.T) {
	t.Run("Ensure unlock succeed", func(t *testing.T) {
		prov, mockLck := setup(t)

		mockLck.EXPECT().UnLock().Return(nil)
		l := NewCustomLock("foo.lck", prov)

		err := l.UnLock()
		assert.Equal(t, nil, err)
	})
	t.Run("Ensure unlock (fcntl lock) succeed even if file is not locked", func(t *testing.T) {
		lockfile, tfCleanup := test_helper.TempFile(t)
		defer tfCleanup()
		l := New(lockfile)

		err := l.UnLock()
		assert.Equal(t, nil, err)
	})
}
