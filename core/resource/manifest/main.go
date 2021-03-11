package manifest

import (
	"opensvc.com/opensvc/core/keywords"
)

type (
	//
	// Type describes a driver so callers can format the input as the
	// driver expects.
	//
	Type struct {
		Group    string             `json:"group"`
		Name     string             `json:"name"`
		Keywords []keywords.Keyword `json:"keywords"`
	}
)
