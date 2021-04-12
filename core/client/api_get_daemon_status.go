package client

import (
	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/util/funcopt"
)

type GetDaemonStatus struct {
	cli       Getter
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

func NewGetDaemonStatus(cli Getter, opts ...funcopt.O) (*GetDaemonStatus, error) {
	options := &GetDaemonStatus{
		cli:       cli,
		namespace: "",
		selector:  "*",
		relatives: false,
	}
	if err := funcopt.Apply(options, opts...); err != nil {
		return nil, err
	}
	return options, nil
}

// GetDaemonStatus fetchs the daemon status structure from the agent api
func (t *GetDaemonStatus) Get() ([]byte, error) {
	req := request.New()
	req.Action = "daemon_status"
	req.Options["namespace"] = t.namespace
	req.Options["selector"] = t.selector
	req.Options["relatives"] = t.relatives
	return t.cli.Get(*req)
}
