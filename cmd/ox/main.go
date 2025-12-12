package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/opensvc/om3/v3/core/ox"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			filename := filepath.Join(rawconfig.Paths.Var, "ox.stack")
			if f, err := os.Create(filename); err == nil {
				defer f.Close()
				fmt.Fprintf(f, "panic: %s\n\n", r)
				fmt.Fprint(f, string(debug.Stack()))
			}
			panic(r)
		}
	}()

	ox.Execute()
}
