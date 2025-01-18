package oxcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	CmdNodeEvents struct {
		OptsGlobal
		Filters  []string
		Duration time.Duration
		Limit    uint64
		Template string
		Wait     bool

		templ  *template.Template
		helper *templateHelper

		cli          *client.T
		NodeSelector string
		errC         chan error
		evC          chan *event.Event
	}

	templateHelper struct {
		PassedMap map[string]struct{}
		Success   bool
	}
)

var (
	errEventRead = fmt.Errorf("event read error")
)

func (c *templateHelper) passSet(s string, b bool) (changed bool) {
	if b {
		return c.passAdd(s)
	} else {
		return c.passDel(s)
	}
}

func (c *templateHelper) passAdd(s string) (changed bool) {
	if _, ok := c.PassedMap[s]; !ok {
		changed = true
		c.PassedMap[s] = struct{}{}
	}
	return changed
}

func (c *templateHelper) passDel(s string) (changed bool) {
	if _, ok := c.PassedMap[s]; ok {
		changed = true
		delete(c.PassedMap, s)
	}
	return changed
}

func (c *templateHelper) passCount() int {
	return len(c.PassedMap)
}

func (c *templateHelper) setSuccess(b bool) bool {
	c.Success = b
	return b
}

func toInst(p naming.Path, n string) string {
	return p.String() + "@" + n
}

func hasNodeLabel(labels []pubsub.Label, expected ...string) bool {
	for _, l := range labels {
		for _, n := range expected {
			search := pubsub.Label{"node", n}
			if l == search {
				return true
			}
		}
	}
	return false
}

func hasPathLabel(labels []pubsub.Label, expected ...string) bool {
	for _, n := range expected {
		for _, l := range labels {
			search := pubsub.Label{"path", n}
			if l == search {
				return true
			}
		}
	}
	return false
}

func hasInstanceLabel(labels []pubsub.Label, expected ...string) bool {
	for _, i := range expected {
		s := strings.Split(i, "@")
		switch len(s) {
		case 2:
			m := 0
			for _, l := range labels {
				search := pubsub.Label{"path", s[0]}
				if l == search {
					m++
				} else {
					search := pubsub.Label{"node", s[1]}
					if l == search {
						m++
					}
				}
			}
			if m == 2 {
				return true
			}
		}
	}
	return false
}

func (t *CmdNodeEvents) Run() error {
	if t.Wait && t.Limit == 0 {
		t.Limit = 1
	}
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.doNodes()
}

func (t *CmdNodeEvents) doNodes() error {
	var (
		err       error
		now       = time.Now()
		nodenames []string
	)
	t.evC = make(chan *event.Event)
	if t.Template != "" {
		t.Output = "json"
		evTemplate := template.New("ev")
		t.helper = &templateHelper{PassedMap: make(map[string]struct{})}

		funcMap := template.FuncMap{
			"hasNodeLabel":     hasNodeLabel,
			"hasPathLabel":     hasPathLabel,
			"hasInstanceLabel": hasInstanceLabel,
			"toInst":           toInst,
			"passSet":          t.helper.passSet,
			"passAdd":          t.helper.passAdd,
			"passDel":          t.helper.passDel,
			"passCount":        t.helper.passCount,
			"setSuccess":       t.helper.setSuccess,
		}

		evTemplate.Funcs(funcMap)
		t.templ, err = evTemplate.Parse(t.Template)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "parse template error '%s'\n", err)
			return err
		}
	}
	t.cli, err = client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	nodenames, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(t.cli)).Expand()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if t.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Duration)
		defer cancel()
	}
	for _, nodename := range nodenames {
		go t.nodeEventLoop(ctx, nodename)
	}

	var count uint64
	for {
		for {
			select {
			case <-ctx.Done():
				if t.templ != nil && t.Wait && !t.helper.Success {
					err := fmt.Errorf("wait failed after %s (%s)", time.Now().Sub(now), ctx.Err())
					return err
				}
				return ctx.Err()
			case ev := <-t.evC:
				count++
				t.doEvent(*ev)
				if t.templ != nil && t.Wait && t.helper.Success {
					s := fmt.Sprintf("wait succeed elapsed %s", time.Now().Sub(now))
					if len(t.helper.PassedMap) > 0 {
						ok := make([]string, 0)
						for k := range t.helper.PassedMap {
							ok = append(ok, k)
						}
						_, _ = fmt.Fprintf(os.Stderr, "%s passed %s\n", s, ok)
					} else {
						_, _ = fmt.Fprintf(os.Stderr, "%s\n", s)
					}
					return nil
				}
				if t.Limit > 0 && count >= t.Limit {
					if t.templ != nil && t.Wait && !t.helper.Success {
						err := fmt.Errorf("wait failed after %s (event count limit)", time.Now().Sub(now))
						return err
					}
					_, _ = fmt.Fprintf(os.Stderr, "wait comleted after %s\n", time.Now().Sub(now))
					return nil
				}
			case _ = <-t.errC:
				// TODO: verify if we can drop nodeEventLoop errors
			}
		}
	}
}

func (t *CmdNodeEvents) nodeEventLoop(ctx context.Context, nodename string) {
	var (
		retries    = 0
		maxRetries = 600
	)

	evReader, err := t.getEvReader(nodename)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "getEvReader %s: %s", nodename, err)
	}
	if t.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Duration)
		defer cancel()
	}
	defer func() {
		if evReader == nil {
			return
		} else if err := evReader.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close event reader error %s: '%s'\n", nodename, err)
		}
	}()

	for {
		var ev *event.Event
		for { // read loop
			select {
			case <-ctx.Done():
				t.errC <- ctx.Err()
				return
			default:
			}
			if evReader == nil {
				break
			}
			ev, err = evReader.Read()
			if err != nil {
				break
			}
			t.evC <- ev
		}
		for { // get reader retry loop
			select {
			case <-ctx.Done():
				t.errC <- ctx.Err()
				return
			default:
			}
			retries++
			if retries > maxRetries {
				t.errC <- errEventRead
				return
			} else if retries == 1 {
				_, _ = fmt.Fprintf(os.Stderr, "event read failed for node %s: '%s'\n", nodename, err)
				_, _ = fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
			}
			time.Sleep(1 * time.Second)
			select {
			case <-ctx.Done():
				t.errC <- ctx.Err()
				return
			default:
			}
			evReader, err = t.getEvReader(nodename)
			if err == nil {
				_, _ = fmt.Fprintf(os.Stderr, "retry %d of %d ok for %s\n", retries, maxRetries, nodename)
				retries = 0
				break
			}
			_, _ = fmt.Fprintf(os.Stderr, "retry %d of %d failed for %s: '%s'\n", retries, maxRetries, nodename, err)
		}
	}
}

func (t *CmdNodeEvents) getEvReader(nodename string) (event.ReadCloser, error) {
	return t.cli.NewGetEvents().
		SetRelatives(false).
		SetLimit(t.Limit).
		SetWait(t.Wait).
		SetFilters(t.Filters).
		SetDuration(t.Duration).
		SetNodename(nodename).
		SetSelector(t.ObjectSelector).
		GetReader()
}

func (t *CmdNodeEvents) doEvent(e event.Event) {
	msg, err := msgbus.EventToMessage(e)
	if err != nil {
		return
	}
	ce := e.AsConcreteEvent(msg)
	if t.templ != nil {
		var i any
		if b, err := json.Marshal(ce); err != nil {
			return
		} else if err := json.Unmarshal(b, &i); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "unmarshal event data in a any-typed variable: %s\n", err)
			return
		}
		if err := t.templ.Execute(os.Stdout, i); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "template execute error %s\n", err)
		}
		return
	}
	if t.Output == output.JSON.String() {
		t.Output = output.JSONLine.String()
	}
	output.Renderer{
		Output:   t.Output,
		Color:    t.Color,
		Data:     ce,
		Colorize: rawconfig.Colorize,
		Stream:   true,
	}.Print()
}
