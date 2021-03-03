package main

import (
	"opensvc.com/opensvc/core/check"
)

// Type is the check type
type Type check.Type

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "fs_i"
	// DriverName is the name of check driver.
	DriverName = "df"
)

func main() {
	var t Type
	check.Check(t)
}
