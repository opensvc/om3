package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemoncmd"
)

type (
	CmdNodePing struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePing) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}

	sel := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c))
	nodenames, err := sel.Expand()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*1)
	defer cancel()

	q := make(chan daemoncmd.PingItem)
	errC := make(chan error)
	doneC := make(chan string)
	todo := len(nodenames)

	for _, nodename := range nodenames {
		go func(nodename string) {
			ctx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
			defer cancel()
			defer func() { doneC <- nodename }()
			d := daemoncmd.PingItem{
				Data: daemoncmd.Ping{Ping: false},
				Meta: daemoncmd.NodeMeta{Node: nodename},
			}
			begin := time.Now()
			response, err := c.GetNodePingWithResponse(ctx, nodename)
			if errors.Is(err, context.DeadlineExceeded) {
				d.Data.Detail = err.Error()
				q <- d
				return
			}
			if err != nil {
				errC <- err
				return
			}
			switch {
			case response.StatusCode() == http.StatusNoContent:
				d.Data.Ping = true
				d.Data.Detail = fmt.Sprintf("RTT : %s", time.Now().Sub(begin))
				q <- d
			default:
				d.Data.Detail = fmt.Sprintf("%s: %s", nodename, response.Status())
				q <- d
			}
		}(nodename)
	}

	var (
		errs error
		done int
		data daemoncmd.PingItems
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case d := <-q:
			data = append(data, d)
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

	defaultOutput := "tab=NODE:meta.node,PING:data.ping,DETAIL:data.detail"
	output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()

	return errs
}
