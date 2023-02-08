package api

import (
	"time"

	"github.com/opensvc/om3/core/client/request"
)

// PostDaemonAuth describes the daemon auth api handler options.
type PostDaemonAuth struct {
	Base
	Server   string        `json:"server"`
	Roles    []string      `json:"roles"`
	Duration time.Duration `json:"duration"`
}

// NewPostDaemonAuth allocates a PostDaemonAuth struct and sets
// default values to its keys.
func NewPostDaemonAuth(t Getter) *PostDaemonAuth {
	r := &PostDaemonAuth{
		Duration: 10 * time.Minute,
	}
	r.SetClient(t)
	r.SetAction("/auth/token")
	r.SetMethod("POST")
	return r
}

// Do auth token
func (t PostDaemonAuth) Do() ([]byte, error) {
	req := request.NewFor(t)
	for _, role := range t.Roles {
		req.Values.Add("role", role)
	}
	req.Values.Add("duration", t.Duration.String())
	return Route(t.client, *req)
}

func (t *PostDaemonAuth) SetDuration(duration time.Duration) *PostDaemonAuth {
	t.Duration = duration
	return t
}

func (t *PostDaemonAuth) SetRoles(roles []string) *PostDaemonAuth {
	t.Roles = roles
	return t
}
