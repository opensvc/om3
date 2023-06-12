package request

import (
	"encoding/json"
	"net/url"
)

type (
	// T is a api request abstracting the protocol differences
	T struct {
		Method  string                 `json:"method,omitempty" yaml:"method,omitempty"`
		Action  string                 `json:"action,omitempty" yaml:"action,omitempty"`
		Node    string                 `json:"node,omitempty" yaml:"node,omitempty"`
		Options map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`
		Values  url.Values             `json:"query_args,omitempty" yaml:"query_args,omitempty"`
	}
)

// New allocates an unconfigured request and returns its
// address.
func New() *T {
	return &T{
		Options: make(map[string]interface{}),
		Values:  url.Values{},
	}
}

type Optioner interface {
	GetAction() string
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
