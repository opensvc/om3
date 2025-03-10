package oxcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdNodeConfigShow struct {
		OptsGlobal
		Eval         bool
		Impersonate  string
		NodeSelector string
	}
)

func (t *CmdNodeConfigShow) Run() error {

	var (
		data      result
		err       error
		nodenames []string
	)

	c, err := client.New()
	if err != nil {
		return err
	}

	if t.NodeSelector == "" {
		if !clientcontext.IsSet() {
			nodenames = []string{hostname.Hostname()}
		} else {
			return fmt.Errorf("--node must be specified")
		}
	} else {
		l, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
		nodenames = l
	}
	if len(nodenames) == 0 {
		return fmt.Errorf("no match")
	}

	data, err = t.extract(nodenames, c)

	var render func() string
	if len(nodenames) > 1 {
		render = func() string {
			s := ""
			for nodename, d := range data {
				s += "#\n"
				s += "# nodename: " + nodename + "\n"
				s += "#\n"
				s += strings.Repeat("#", 78) + "\n"
				s += d.Render()
			}
			return s
		}
		output.Renderer{
			Output:        t.Output,
			Color:         t.Color,
			Data:          data,
			HumanRenderer: render,
			Colorize:      rawconfig.Colorize,
		}.Print()
	} else {
		nodename := nodenames[0]
		// single node selection
		render = func() string {
			if d, ok := data[nodename]; ok {
				return d.Render()
			}
			return ""
		}
		output.Renderer{
			Output:        t.Output,
			Color:         t.Color,
			Data:          data,
			HumanRenderer: render,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}
	return err
}

func (t *CmdNodeConfigShow) extract(nodenames []string, c *client.T) (result, error) {

	data := make(result)

	todo := len(nodenames)
	if todo == 0 {
		return data, nil
	}

	errC := make(chan error)
	doneC := make(chan string)
	ctx := context.Background()

	for _, nodename := range nodenames {
		go func(nodename string) {
			defer func() { doneC <- nodename }()
			if d, err := t.extractFromDaemon(ctx, nodename, c); err != nil {
				errC <- err
				return
			} else {
				data[nodename] = d
			}
		}(nodename)
	}

	var (
		errs error
		done int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case <-doneC:
			done++
			if done == todo {
				return data, errs
			}
		}
	}
}

func (t *CmdNodeConfigShow) extractFromDaemon(ctx context.Context, nodename string, c *client.T) (rawconfig.T, error) {
	params := api.GetNodeConfigParams{}
	if t.Eval {
		v := true
		params.Evaluate = &v
	}
	if t.Impersonate != "" {
		params.Impersonate = &t.Impersonate
	}

	data := rawconfig.T{}

	response, err := c.GetNodeConfigWithResponse(ctx, nodename, &params)
	if err != nil {
		return data, err
	}
	switch {
	case response.JSON200 != nil:
		if b, err := json.Marshal(response.JSON200.Data); err != nil {
			return data, err
		} else if err = json.Unmarshal(b, &data); err != nil {
			return data, err
		} else {
			return data, nil
		}
	case response.JSON401 != nil:
		return data, fmt.Errorf("%s: %s", nodename, *response.JSON401)
	case response.JSON403 != nil:
		return data, fmt.Errorf("%s: %s", nodename, *response.JSON403)
	default:
		return data, fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
	}
}
