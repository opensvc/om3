package test_helper

import (
	"io/ioutil"
	"os"
	"testing"
)

func Tempdir(t *testing.T) (td string, tdCleanup func()) {
	t.Helper()

	var err error
	if td, err = ioutil.TempDir("", "testdir"); err != nil {
		t.Fatalf("ioutil.TempDir error= %v", err)
	}
	tdCleanup = func() {
		return
	}
	return td, tdCleanup
}

func TempFile(t *testing.T, dirNames ...string) (string, func()) {
	t.Helper()
	var dir string
	if dirNames == nil {
		dir = ""
	} else {
		dir = dirNames[0]
	}
	tf, err := ioutil.TempFile(dir, "test")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	_, _ = tf.WriteString("#!/bin/bash\n")
	if err = tf.Close(); err != nil {
		t.Fatalf("TempFile close error: %v", err)
	}

	cleanup := func() {
		if _, err = os.Stat(tf.Name()); err != nil && os.IsNotExist(err) {
			return
		}
		if err = os.Remove(tf.Name()); err != nil {
			t.Fatalf("TempFile cleanup error: %v", err)
		}
	}
	return tf.Name(), cleanup
}

func TempFileExec(t *testing.T, dirNames ...string) (string, func()) {
	t.Helper()

	tf, tcCleanup := TempFile(t, dirNames...)

	if err := os.Chmod(tf, 0555); err != nil {
		t.Fatalf("TempFileExec Chmod error: %v", err)
	}

	return tf, tcCleanup
}
