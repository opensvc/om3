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
		OptsGlobal
		Verbose bool   `flag:"poolstatusverbose"`
		Name    string `flag:"poolstatusname"`
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
	if !t.Local || clientcontext.IsSet() {
		data, err = t.extractDaemon()
	} else {
		data, err = t.extractLocal()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     data,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return data.Render(t.Verbose)
		},
	}.Print()
}

func (t *PoolStatus) extractLocal() (pool.StatusList, error) {
	if t.Name == "" {
		return object.NewNode().ShowPools(), nil
	} else {
		return object.NewNode().ShowPoolsByName(t.Name), nil
	}
}

func (t *PoolStatus) extractDaemon() (pool.StatusList, error) {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return nil, err
	}
	l := pool.NewStatusList()
	data := make(map[string]pool.Status)
	req := c.NewGetPools()
	req.SetName(t.Name)
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
