package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/client"
)

type (
	CmdDaemonAuth struct {
		OptsGlobal
		Roles    []string
		Duration time.Duration
		Out      []string
	}
)

func (t *CmdDaemonAuth) Run() error {
	c, err := client.New(
		client.WithURL(t.Server),
	)
	if err != nil {
		return err
	}
	req := c.NewPostDaemonAuth().
		SetDuration(t.Duration).
		SetRoles(t.Roles)
	var b []byte
	b, err = req.Do()
	if err != nil {
		return err
	}
	if len(t.Out) == 0 {
		_, err = fmt.Fprintf(os.Stdout, "%s\n", b)
		if err != nil {
			return err
		}
	} else {
		var parsed map[string] interface{}
		if err := json.Unmarshal(b, &parsed); err != nil {
			return err
		}
		for _, out := range t.Out {
			if v, ok := parsed[out]; ok {
				_, err := fmt.Printf("%s\n", v)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
