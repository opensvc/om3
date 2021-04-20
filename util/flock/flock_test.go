package flock

import (
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/test_helper"
	"testing"
	"time"
)

func TestUnLock(t *testing.T) {
	t.Run("Ensure unlock succeed even if file doesn't exists", func(t *testing.T) {
		td, tfCleanup := test_helper.Tempdir(t)
		defer tfCleanup()
		orig := getPathVar
		getPathVar = func() string { return td }
		defer func() { getPathVar = orig }()

		assert.Equal(t, nil, New("foo.lock").UnLock())
	})
}

func TestLockUnLock(t *testing.T) {
	t.Run("test Lock, then Unlock", func(t *testing.T) {
		td, tfCleanup := test_helper.Tempdir(t)
		defer tfCleanup()
		orig := getPathVar
		getPathVar = func() string { return td }
		defer func() { getPathVar = orig }()
		l := New("foo.lock")
		println(l.Path)
		assert.Equal(t, nil, l.Lock(10000*time.Millisecond, "test-lock-unlock"))
		assert.Equal(t, nil, l.UnLock())
	})
}
