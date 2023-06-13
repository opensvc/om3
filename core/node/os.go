package node

import "github.com/opensvc/om3/util/san"

type (
	// Os defines Os details
	Os struct {
		Paths san.Paths `json:"paths" yaml:"paths"`
	}
)
