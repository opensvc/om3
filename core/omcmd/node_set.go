package omcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdNodeSet struct {
		OptsGlobal
		OptsLock
		KeywordOps   []string
		NodeSelector string
	}
)

func (t *CmdNodeSet) Run() error {
	if t.Local {
		t.NodeSelector = hostname.Hostname()
	}
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}

	data, err := t.doRemote()

	if len(data) != 0 {
		defaultOutput := "tab=NODE:meta.node,ISCHANGED:data.changed"
		output.Renderer{
			DefaultOutput: defaultOutput,
			Output:        t.Output,
			Color:         t.Color,
			Data:          data,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	return err
}

func (t *CmdNodeSet) doRemote() ([]api.IsChangedItem, error) {
	l := make([]api.IsChangedItem, 0)

	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return l, err
	}
	params := api.PostNodeConfigUpdateParams{}
	params.Set = &t.KeywordOps
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if len(nodenames) == 0 {
		return l, fmt.Errorf("no match")
	}
	if err != nil {
		return l, err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	q := make(chan api.IsChangedItem)
	errC := make(chan error)
	doneC := make(chan string)
	todo := len(nodenames)

	var (
		needDoLocal bool
		done        int
	)

	for _, nodename := range nodenames {
		if nodename == hostname.Hostname() {
			needDoLocal = true
			done++
			continue
		}
		go func(nodename string) {
			defer func() { doneC <- nodename }()
			response, err := c.PostNodeConfigUpdateWithResponse(context.Background(), nodename, &params)
			if err != nil {
				errC <- err
				return
			}
			switch response.StatusCode() {
			case 200:
				data := *response.JSON200
				data.Meta.Node = nodename
				q <- data
			case 400:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON400)
			case 401:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON401)
			case 403:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON403)
			case 500:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON500)
			default:
				errC <- fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
			}
		}(nodename)
	}

	var (
		errs error
	)

	for {
		if todo == 1 && needDoLocal {
			goto out
		}
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case isChanged := <-q:
			l = append(l, isChanged)
		case <-doneC:
			done++
			if done == todo {
				goto out
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			goto out
		}
	}

out:

	if needDoLocal {
		data, err := t.doLocal()
		if err == nil {
			l = append(l, data)
		}
		errs = errors.Join(errs, err)
	}

	return l, errs
}

func (t *CmdNodeSet) doLocal() (api.IsChangedItem, error) {
	var data api.IsChangedItem
	data.Meta.Node = hostname.Hostname()

	n, err := object.NewNode()
	if err != nil {
		return data, err
	}

	isChanged, err := n.Config().UpdateAndReportIsChanged([]string{}, []key.T{}, keyop.ParseOps(t.KeywordOps))
	if err != nil {
		return data, err
	}

	data.Data.Ischanged = isChanged

	return data, err
}
