package request

import (
	"encoding/json"
)

type (
	// T is a api request abstracting the protocol differences
	T struct {
		Method    string                 `json:"method,omitempty"`
		Action    string                 `json:"action,omitempty"`
		Node      string                 `json:"node,omitempty"`
		Options   map[string]interface{} `json:"options,omitempty"`
		QueryArgs map[string]string      `json:"query_args,omitempty"`
	}
)

// New allocates an unconfigured request and returns its
// address.
func New() *T {
	r := &T{}
	r.Options = make(map[string]interface{})
	r.QueryArgs = make(map[string]string)
	return r
}

type Optioner interface {
	GetAction() string
	GetQueryArgs() map[string]string
	GetNode() string
	GetMethod() string
}

// NewFor allocates a fully configured request and returns its
// address.
func NewFor(options Optioner) *T {
	var (
		b   []byte
		err error
	)
	r := New()
	r.Action = options.GetAction()
	r.Node = options.GetNode()
	r.Method = options.GetMethod()
	r.QueryArgs = options.GetQueryArgs()
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
