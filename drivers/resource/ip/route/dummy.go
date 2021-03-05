// +build !linux

package main

import (
	"fmt"
	"opensvc.com/opensvc/core/status"
)

// Start the Resource
func (r Type) Start() error {
	return nil
}

// Stop the Resource
func (r Type) Stop() error {
	return nil
}

// Label returns a formatted short description of the Resource
func (r Type) Label() string {
	return fmt.Sprintf("%s via %s", r.Destination, r.Gateway)
}

// Status evaluates and display the Resource status and logs
func (r Type) Status() status.Type {
	//r.Log.Error("not implemented on %s", runtime.GOOS)
	return status.NotApplicable
}
