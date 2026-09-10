package omcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNodeCapabilitiesList struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeCapabilitiesList) extract(c *client.T, nodename string) (api.CapabilityList, error) {
	resp, err := c.GetNodeCapabilitiesWithResponse(context.Background(), nodename)
	if err != nil {
		return api.CapabilityList{}, err
	}
	switch resp.StatusCode() {
	case 200:
		return *resp.JSON200, nil
	case 401:
		return api.CapabilityList{}, fmt.Errorf("%s: %s", nodename, *resp.JSON401)
	case 403:
		return api.CapabilityList{}, fmt.Errorf("%s: %s", nodename, *resp.JSON403)
	default:
		return api.CapabilityList{}, fmt.Errorf("%s: unexpected statuscode: %s", nodename, resp.Status())
	}
}

func (t *CmdNodeCapabilitiesList) remote() error {
	var errs error
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	var data api.CapabilityList
	for i, nodename := range nodenames {
		if d, err := t.extract(c, nodename); err != nil {
			errs = errors.Join(err)
		} else if i == 0 {
			data = d
		} else {
			data.Items = append(data.Items, d.Items...)
		}

	}
	output.Renderer{
		DefaultOutput: "tab=NODE:meta.node,NAME:data.name",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}

func (t *CmdNodeCapabilitiesList) Run() error {
	if t.isThisNodeOnly() {
		return t.local()
	}
	return t.remote()
}

// isThisNodeOnly reports whether the selector asks for this node and nothing
// else.
//
// The capabilities of a node are read from the node that scanned them, so
// that question is answered from the file here and the daemon is never asked.
// An empty selector is the same question: it is what a caller building this
// command otherwise than from a command line leaves behind, where the option
// carries the name of this node.
func (t *CmdNodeCapabilitiesList) isThisNodeOnly() bool {
	return t.NodeSelector == "" || t.NodeSelector == hostname.Hostname()
}

func (t *CmdNodeCapabilitiesList) local() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	caps, err := n.PrintCapabilities()
	if err != nil {
		return err
	}
	data := api.CapabilityList{
		Kind: "CapabilityList",
	}
	localhost := hostname.Hostname()
	for _, e := range caps {
		item := api.CapabilityItem{
			Kind: "CapabilityItem",
			Meta: api.NodeMeta{
				Node: localhost,
			},
			Data: api.Capability{
				Name: e,
			},
		}
		data.Items = append(data.Items, item)
	}
	output.Renderer{
		DefaultOutput: "tab=data.name",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
