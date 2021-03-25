package status

import (
	"opensvc.com/opensvc/core/client"
)

type getDaemonStatus struct {
	cli       client.Getter `json:"-"`
	namespace string        `json:"namespace,omitempty"`
	selector  string        `json:"selector,omitempty"`
	relatives bool          `json:"relatives,omitempty"`
}

func New(cli client.Getter, selector string) *getDaemonStatus {
	return &getDaemonStatus{
		cli,
		"",
		selector,
		false,
	}
}

// GetDaemonStatus fetchs the daemon status structure from the agent api
func (c *getDaemonStatus) Get() ([]byte, error) {
	request := c.newRequest()
	return c.cli.Get(*request)
}

func (c *getDaemonStatus) newRequest() *client.Request {
	request := client.NewRequest()
	request.Action = "daemon_status"
	request.Options["namespace"] = c.namespace
	request.Options["selector"] = c.selector
	request.Options["relatives"] = c.relatives
	return request
}
