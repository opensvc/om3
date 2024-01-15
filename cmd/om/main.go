package main

import (
	"os"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/om"
)

func main() {
	if err := os.Unsetenv(env.ContextVar); err != nil {
		panic(err)
	}
	om.Execute()
}
