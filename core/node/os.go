package node

import "opensvc.com/opensvc/util/san"

type (
	// Os defines Os details
	Os struct {
		Paths san.Paths `json:"paths"`
	}
)
