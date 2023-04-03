package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/daemon/api"
)

// PostNodeFile sends a file content of a supported kind, for the
// daemon to write it in a well-known location.
type PostNodeFile struct {
	Base
	api.PostNodeFileParams
	api.ObjectFile
}

func NewPostNodeFile(t Poster) *PostNodeFile {
	r := &PostNodeFile{}
	r.SetClient(t)
	r.SetMethod("POST")
	r.SetAction("/node/file")
	return r
}

// Do ...
func (t PostNodeFile) Do() ([]byte, error) {
	req := request.NewFor(t)
	req.Options["data"] = t.Data
	req.Values.Add("name", t.Name)
	req.Values.Add("kind", t.Kind)
	return Route(t.client, *req)
}
