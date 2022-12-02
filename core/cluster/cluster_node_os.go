package cluster

import "opensvc.com/opensvc/util/san"

type (
	// NodeOs defines Os details
	NodeOs struct {
		Paths san.Paths `json:"paths"`
	}
)
