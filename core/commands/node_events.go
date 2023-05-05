package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
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
		templ    *template.Template
		helper   *templateHelper
	}

	templateHelper struct {
		PassedMap map[string]struct{}
		Success   bool
	}
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

func toInst(p path.T, n string) string {
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
	var (
		err        error
		c          *client.T
		ev         *event.Event
		maxRetries = 600
		retries    = 0
		now        = time.Now()
	)
	if t.Template != "" {
		t.Format = "json"
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
	c, err = client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}

	getEvents := c.NewGetEvents().
		SetRelatives(false).
		SetLimit(t.Limit).
		SetFilters(t.Filters)
	ctx := context.Background()
	if t.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Duration)
		defer cancel()
		getEvents.SetDuration(t.Duration)
	}

	evReader, err := getEvents.GetReader()
	if err != nil {
		return err
	}
	var count uint64
	for {
		for {
			select {
			case <-ctx.Done():
				if t.templ != nil && t.Wait && !t.helper.Success {
					err := errors.Errorf("wait failed after %s (%s)\n", time.Now().Sub(now), ctx.Err())
					return err
				}
				return ctx.Err()
			default:
			}
			ev, err = evReader.Read()
			if err != nil {
				break
			}
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
					err := errors.Errorf("wait failed after %s (event count limit)\n", time.Now().Sub(now))
					return err
				}
				return nil
			}
		}
		if err1 := evReader.Close(); err1 != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close event reader error '%s'\n", err1)
			return err
		}
		select {
		case <-ctx.Done():
			if t.templ != nil && t.Wait && !t.helper.Success {
				err := errors.Errorf("wait failed after %s (%s)\n", time.Now().Sub(now), ctx.Err())
				return err
			}
			return ctx.Err()
		default:
		}
		for {
			retries++
			if retries > maxRetries {
				if t.templ != nil && t.Wait && !t.helper.Success {
					err := errors.Errorf("wait failed after %s (max retries)\n", time.Now().Sub(now))
					return err
				}
				return err
			} else if retries == 1 {
				_, _ = fmt.Fprintf(os.Stderr, "event read failed: '%s'\n", err)
				_, _ = fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
			}
			time.Sleep(1 * time.Second)
			select {
			case <-ctx.Done():
				if t.templ != nil && t.Wait && !t.helper.Success {
					err := errors.Errorf("wait failed after %s (max retries)\n", time.Now().Sub(now))
					return err
				}
				return ctx.Err()
			default:
			}
			evReader, err = c.NewGetEvents().
				SetRelatives(false).
				SetLimit(t.Limit).
				SetFilters(t.Filters).
				SetDuration(t.Duration).
				GetReader()
			if err == nil {
				_, _ = fmt.Fprintf(os.Stderr, "retry %d of %d ok\n", retries, maxRetries)
				retries = 0
				break
			}
			_, _ = fmt.Fprintf(os.Stderr, "retry %d of %d failed: '%s'\n", retries, maxRetries, err)
		}
	}
}

func (t *CmdNodeEvents) doEvent(e event.Event) {
	if t.templ != nil {
		msg, err := msgbus.EventToMessage(e)
		if err != nil {
			return
		}
		msg.GetLabels()
		if err := t.templ.Execute(os.Stdout, msg); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "template execute error %s\n", err)
		}
		return
	}
	human := func() string {
		return event.Render(e)
	}
	if t.Format == output.JSON.String() {
		t.Format = output.JSONLine.String()
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          e,
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
