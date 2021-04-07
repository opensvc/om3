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

func setup(t *testing.T) (ctrl *gomock.Controller, prov *mockFcntlLock.MockLockProvider, lck *mockFcntlLock.MockLocker) {
	ctrl = gomock.NewController(t)
	prov = mockFcntlLock.NewMockLockProvider(ctrl)
	lck = mockFcntlLock.NewMockLocker(ctrl)

	prov.EXPECT().New(gomock.Any()).Return(lck)
	return
}

func TestLock(t *testing.T) {
	t.Run("Ensure write data to lock file when lock succeed", func(t *testing.T) {
		_, prov, lck := setup(t)

		var b []byte
		lck.EXPECT().LockContext(gomock.Any(), 500*time.Millisecond).Return(nil)
		lck.EXPECT().
			Write(gomock.AssignableToTypeOf(b)).
			Do(func(arg []byte) {
				b = arg
			}).
			Return(0, nil)

		err := NewCustomLock("foo.lck", prov).Lock(100*time.Millisecond, "intent1")
		assert.Equal(t, nil, err)

		found := meta{}
		if err := json.Unmarshal(b, &found); err != nil {
			t.Fatalf("expected written data : %+v\n", b)
		}
		assert.Equal(t, "intent1", found.Intent)
	})

	t.Run("Ensure return error if lock fail", func(t *testing.T) {
		_, prov, lck := setup(t)

		lck.EXPECT().LockContext(gomock.Any(), gomock.Any()).Return(errors.New("can't lock"))

		err := NewCustomLock("foo.lck", prov).Lock(100*time.Millisecond, "intent1")
		assert.Equal(t, errors.New("can't lock"), err)
	})

	t.Run("lockfile is created", func(t *testing.T) {
		lockfile, tfCleanup := test_helper.TempFile(t)
		defer tfCleanup()
		l := New(lockfile)
		err := l.Lock(1*time.Second, "plop")
		assert.Equal(t, nil, err)
		data, err := os.ReadFile(lockfile)
		assert.Greater(t, len(data), 50)
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
		_, prov, lck := setup(t)

		lck.EXPECT().UnLock().Return(nil)
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
