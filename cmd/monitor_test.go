package cmd

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func Test_ExecuteCommand(t *testing.T) {
	var err error
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	stderr := bytes.NewBufferString("")
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"monitor", "--help"})

	if err = rootCmd.Execute(); err != nil {
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
