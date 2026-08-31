package node

import "github.com/opensvc/om3/v3/util/san"

type (
	// Os defines Os details
	Os struct {
		Paths san.Paths `json:"paths"`
	}
)

// DeepCopy returns a copy of the os details sharing nothing with them.
//
// A nil Paths is kept nil rather than made empty: the field has no
// omitempty, so the two do not serialize alike.
func (t Os) DeepCopy() Os {
	if t.Paths == nil {
		return Os{}
	}
	return Os{Paths: *t.Paths.DeepCopy()}
}
