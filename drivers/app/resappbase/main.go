package resappbase

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
)

// T is the app base driver structure
type T struct {
	resource.T
	RetCodes string   `json:"retcodes"`
	Path     path.T   `json:"path"`
	Nodes    []string `json:"nodes"`
}
