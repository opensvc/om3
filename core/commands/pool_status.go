package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// PoolStatus is the cobra flag set of the command.
	PoolStatus struct {
		Global  object.OptsGlobal
		Verbose bool `flag:"poolstatusverbose"`
		Name    bool `flag:"poolstatusname"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *PoolStatus) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *PoolStatus) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "show the cluster pools usage",
		Aliases: []string{"statu", "stat", "sta", "st"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *PoolStatus) run() {
	var (
		err  error
		data pool.StatusList
	)
	if !t.Global.Local || clientcontext.IsSet() {
		data, err = t.extractDaemon()
	} else {
		data, err = t.extractLocal()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	output.Renderer{
		Format:   t.Global.Format,
		Color:    t.Global.Color,
		Data:     data,
		Colorize: rawconfig.Node.Colorize,
		HumanRenderer: func() string {
			return data.Render()
		},
	}.Print()
}

func (t *PoolStatus) extractLocal() (pool.StatusList, error) {
	return object.NewNode().ShowPools(), nil
}

func (t *PoolStatus) extractDaemon() (pool.StatusList, error) {
	c, err := client.New(client.WithURL(t.Global.Server))
	if err != nil {
		return nil, err
	}
	l := pool.NewStatusList()
	data := make(map[string]pool.Status)
	req := c.NewGetPools()
	b, err := req.Do()
	if err != nil {
		return l, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return l, errors.Wrapf(err, "unmarshal GET /pools")
	}
	for name, d := range data {
		d.Name = name
		l = append(l, d)
	}
	return l, nil
}
