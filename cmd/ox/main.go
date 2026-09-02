package main

import (
	"github.com/opensvc/om3/v3/core/ox"
)

func main() {
	// The recover that used to write ox.stack is gone with the one of
	// the om main: a panic prints its stack to stderr, which is where
	// the operator running this command is looking.
	ox.Execute()
}
