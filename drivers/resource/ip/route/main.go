package main

import (
	"os"
	"opensvc.com/opensvc/core/resource"
)

func main() {
	var r R
	loader := resource.NewLoader(os.Stdin)
	loader.Load(&r)
	resource.Action(r)
}
