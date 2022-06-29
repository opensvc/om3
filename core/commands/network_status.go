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
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// NetworkStatus is the cobra flag set of the command.
	NetworkStatus struct {
		OptsGlobal
		Verbose bool   `flag:"networkstatusverbose"`
		Name    string `flag:"networkstatusname"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NetworkStatus) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NetworkStatus) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "show the cluster networks usage",
		Aliases: []string{"statu", "stat", "sta", "st"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NetworkStatus) run() {
	var (
		err  error
		data network.StatusList
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

func (t *NetworkStatus) extractLocal() (network.StatusList, error) {
	n := object.NewNode()
	return network.ShowNetworksByName(n, t.Name), nil
}

func (t *NetworkStatus) extractDaemon() (network.StatusList, error) {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return nil, err
	}
	l := network.NewStatusList()
	data := make(map[string]network.Status)
	req := c.NewGetNetworks()
	req.SetName(t.Name)
	b, err := req.Do()
	if err != nil {
		return l, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return l, errors.Wrapf(err, "unmarshal GET /networks")
	}
	for name, d := range data {
		if t.Name != "" && name != t.Name {
			// TODO: api handler should honor the name filter set in request
			continue
		}
		d.Name = name
		l = append(l, d)
	}
	return l, nil
}
