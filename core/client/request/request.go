package request

import (
	"encoding/json"
)

type (
	// T is a api request abstracting the protocol differences
	T struct {
		Method  string                 `json:"method,omitempty"`
		Action  string                 `json:"action,omitempty"`
		Node    string                 `json:"node,omitempty"`
		Options map[string]interface{} `json:"options,omitempty"`
	}
)

//
// New allocates an unconfigured request and returns its
// address.
//
func New() *T {
	r := &T{}
	r.Options = make(map[string]interface{})
	return r
}

//
// NewFor allocates a fully configured request and returns its
// address.
//
func NewFor(action string, options interface{}) *T {
	var (
		b   []byte
		err error
	)
	r := New()
	r.Action = action
	// convert options to the expected json format
	if b, err = json.Marshal(options); err != nil {
		return nil
	}
	if err = json.Unmarshal(b, &r.Options); err != nil {
		return nil
	}
	return r
}

func (t T) String() string {
	b, _ := json.Marshal(t)
	return "request" + string(b)
}
