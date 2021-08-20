package cmd

import (
	"bytes"
	"io/ioutil"
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
	_, err = ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ioutil.ReadAll(stderr)
	if err != nil {
		t.Fatal()
	}
}
