package client

import (
	"errors"
	"fmt"
)

type getDaemonStatus struct {
	cli Getter
	*Namespace
	*Selector
	*Relatives
	//cli       Getter `json:"-"`
	//namespace string `json:"namespace,omitempty"`
	//selector  string `json:"selector,omitempty"`
	//relatives bool   `json:"relatives,omitempty"`
}

func NewGetDaemonStatusB(cli Getter, opts ...OptionExtra) (*getDaemonStatus, error) {
	options := getDaemonStatus{
		cli,
		&Namespace{""},
		&Selector{"*"},
		&Relatives{false},
	}

	for _, o := range opts {
		switch t := o.(type) {
		case SelectorType:
			_ = t.apply(options.Selector)
		case NamespaceType:
			_ = t.apply(options.Namespace)
		case RelativesType:
			_ = t.apply(options.Relatives)
		default:
			message := fmt.Sprintf("non allowed option type %T", t)
			return nil, errors.New(message)
		}
	}
	return &options, nil
}

// GetDaemonStatus fetchs the daemon status structure from the agent api
func (c *getDaemonStatus) Get() ([]byte, error) {
	request := NewRequest()
	request.Action = "daemon_status"
	request.Options["namespace"] = c.NamespaceValue()
	request.Options["selector"] = c.SelectorValue()
	request.Options["relatives"] = c.RelativesValue()
	return c.cli.Get(*request)
}
