package object

import "flag"

// To update golden file of TestInstanceStates_Render, run:
// go test -v ./core/object/... -run TestInstanceStates_Render -update
var update = flag.Bool("update", false, "update golden file fixtures")
