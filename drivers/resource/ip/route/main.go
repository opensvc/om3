package main

import (
	"os"

	"opensvc.com/opensvc/core/resource"
)

func main() {
	r := &Type{}
	resource.NewLoader(os.Stdin).Load(r)
	resource.Action(r)
}
