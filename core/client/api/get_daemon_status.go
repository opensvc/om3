package api

import (
	"opensvc.com/opensvc/core/client/request"
)

type GetDaemonStatus struct {
	client    Getter
	namespace string
	selector  string
	relatives bool
}

func (t *GetDaemonStatus) SetNamespace(s string) *GetDaemonStatus {
	t.namespace = s
	return t
}

func (t *GetDaemonStatus) SetSelector(s string) *GetDaemonStatus {
	t.selector = s
	return t
}

func (t *GetDaemonStatus) SetRelatives(s bool) *GetDaemonStatus {
	t.relatives = s
	return t
}

func (t GetDaemonStatus) Namespace() string {
	return t.namespace
}

func (t GetDaemonStatus) Selector() string {
	return t.selector
}

func (t GetDaemonStatus) Relatives() bool {
	return t.relatives
}

func NewGetDaemonStatus(t Getter) *GetDaemonStatus {
	options := &GetDaemonStatus{
		client:    t,
		namespace: "",
		selector:  "*",
		relatives: false,
	}
	return options
}

// Do fetches the daemon status structure from the agent api
func (t GetDaemonStatus) Do() ([]byte, error) {
	req := request.New()
	req.Action = "/daemon/status"
	req.Options["namespace"] = t.namespace
	req.Options["selector"] = t.selector
	req.Options["relatives"] = t.relatives
	return t.client.Get(*req)
}

func (t GetDaemonStatus) Get() ([]byte, error) {
	return t.Do()
}
