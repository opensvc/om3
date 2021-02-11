package manifest

import (
	"opensvc.com/opensvc/core/keywords"
)

type (
	Type struct {
		Group    string             `json:"group"`
		Name     string             `json:"name"`
		Keywords []keywords.Keyword `json:"keywords"`
	}
)


