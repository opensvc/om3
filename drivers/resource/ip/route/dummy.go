// +build !linux

package main

import (
	"fmt"

	"opensvc.com/opensvc/core/status"
)

// Start the Resource
func (t T) Start() error {
	return nil
}

// Stop the Resource
func (t T) Stop() error {
	return nil
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return fmt.Sprintf("%s via %s", t.Destination, t.Gateway)
}

// Status evaluates and display the Resource status and logs
func (t T) Status() status.T {
	//r.Log.Error("not implemented on %s", runtime.GOOS)
	return status.NotApplicable
}
