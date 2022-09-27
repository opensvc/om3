package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetObjectConfigFile is the options supported by the handler.
type GetObjectConfigFile struct {
	Base
	ObjectSelector string `json:"path"`
}

// NewGetObjectConfigFile allocates a GetObjectConfigFile struct and sets
// default values to its keys.
func NewGetObjectConfigFile(t Getter) *GetObjectConfigFile {
	r := &GetObjectConfigFile{}
	r.SetClient(t)
	r.SetNode("ANY")
	r.SetAction("object/file")
	r.SetMethod("GET")
	return r
}

// Do submits the request.
func (t GetObjectConfigFile) Do() ([]byte, error) {
	t.SetQueryArgs(map[string]string{"path": t.ObjectSelector})
	req := request.NewFor(t)
	return Route(t.client, *req)
}
