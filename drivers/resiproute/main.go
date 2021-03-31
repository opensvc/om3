package resiproute

import (
	"os"

	"opensvc.com/opensvc/core/resource"
)

func main() {
	r := &T{}
	resource.NewLoader(os.Stdin).Load(r)
	resource.Action(r)
}
