package api

import (
	"github.com/opensvc/om3/core/client/request"
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
	req := request.NewFor(t)
	req.Values.Set("path", t.ObjectSelector)
	return Route(t.client, *req)
}
