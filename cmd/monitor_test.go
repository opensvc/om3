package cmd

import (
	"bytes"
	"io"
	"testing"
)

func Test_ExecuteCommand(t *testing.T) {
	var err error
	b := bytes.NewBufferString("")
	root.SetOut(b)
	stderr := bytes.NewBufferString("")
	root.SetErr(stderr)
	root.SetArgs([]string{"monitor", "--help"})

	if err = root.Execute(); err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(stderr)
	if err != nil {
		t.Fatal()
	}
}
