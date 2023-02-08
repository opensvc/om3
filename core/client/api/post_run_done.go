package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// PostRunDone describes the options supported by POST /run_done
type PostRunDone struct {
	Base
	Action string   `json:"action"`
	Path   string   `json:"path"`
	RIDs   []string `json:"rids"`
}

// NewPostRunDone allocates a PostKey struct and sets
// default values to its keys.
func NewPostRunDone(t Poster) *PostRunDone {
	r := &PostRunDone{}
	r.SetClient(t)
	r.SetAction("run_done")
	r.SetMethod("POST")
	return r
}

// Do returns the decoded value of an object key
func (t PostRunDone) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
