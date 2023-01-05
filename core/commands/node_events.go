package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	CmdNodeEvents struct {
		OptsGlobal
		Filters []string
		Duration time.Duration
	}
)

func (t *CmdNodeEvents) Run() error {
	var (
		err        error
		c          *client.T
		ev         *event.Event
		maxRetries = 600
		retries    = 0
	)
	c, err = client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}

	evReader, err := c.NewGetEvents().
		SetRelatives(false).
		SetFilters(t.Filters).
		SetDuration(t.Duration).
		GetReader()
	ctx := context.Background()
	if t.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Duration)
		defer cancel()
	}

	if err != nil {
		return err
	}
	for {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			ev, err = evReader.Read()
			if err != nil {
				break
			}
			t.doEvent(*ev)
		}
		if err1 := evReader.Close(); err1 != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close event reader error '%s'\n", err1)
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		for {
			retries++
			if retries > maxRetries {
				return err
			} else if retries == 1 {
				_, _ = fmt.Fprintf(os.Stderr, "event read failed: '%s'\n", err)
				_, _ = fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
			}
			time.Sleep(1 * time.Second)
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			evReader, err = c.NewGetEvents().
				SetRelatives(false).
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
