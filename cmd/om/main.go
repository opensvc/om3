package main

import (
	"os"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/om"
)

func main() {
	// A recover here used to write the stack of the panicking goroutine
	// to a file. It caught nothing of what crashes the daemon this same
	// binary runs: a panic in any of its other goroutines never reaches
	// it, and a fatal error is not recoverable at all. The daemon
	// arranges its own crash report, see daemon/daemoncmd/crash.go.
	if err := os.Unsetenv(env.ContextVar); err != nil {
		panic(err)
	}
	om.Execute()
}
