package client

import "opensvc.com/opensvc/util/funcopt"

type getDaemonStatus struct {
	cli       Getter
	namespace string
	selector  string
	relatives bool
}

func (t *getDaemonStatus) SetNamespace(s string) {
	t.namespace = s
}

func (t *getDaemonStatus) SetSelector(s string) {
	t.selector = s
}

func (t *getDaemonStatus) SetRelatives(s bool) {
	t.relatives = s
}

func (t getDaemonStatus) Namespace() string {
	return t.namespace
}

func (t getDaemonStatus) Selector() string {
	return t.selector
}

func (t getDaemonStatus) Relatives() bool {
	return t.relatives
}

func NewGetDaemonStatus(cli Getter, opts ...funcopt.O) (*getDaemonStatus, error) {
	options := &getDaemonStatus{
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
func (c *getDaemonStatus) Get() ([]byte, error) {
	request := NewRequest()
	request.Action = "daemon_status"
	request.Options["namespace"] = c.namespace
	request.Options["selector"] = c.selector
	request.Options["relatives"] = c.relatives
	return c.cli.Get(*request)
}
