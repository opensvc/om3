package resiproute

import "fmt"

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return fmt.Sprintf("%s via %s", t.To, t.Gateway)
}
