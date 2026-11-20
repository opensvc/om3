package commoncmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/unstructured"
)

type (
	// CmdNetworkNodeList lists what the last setup of each network made on
	// each node.
	CmdNetworkNodeList struct {
		Color        string
		Output       string
		Sort         string
		Name         string
		NodeSelector string
	}

	// CmdNetworkPeerList lists how each node reaches each other node of
	// each network, as the last setup of the network on the node made it.
	CmdNetworkPeerList struct {
		CmdNetworkNodeList
	}
)

// NewCmdNetworkNode returns the "node" subcommand of network.
func NewCmdNetworkNode() *cobra.Command {
	return &cobra.Command{
		Use:   "node",
		Short: "what the setup of the networks made on the nodes",
	}
}

// NewCmdNetworkPeer returns the "peer" subcommand of network.
func NewCmdNetworkPeer() *cobra.Command {
	return &cobra.Command{
		Use:   "peer",
		Short: "how the nodes reach each other on the networks",
	}
}

func (t *CmdNetworkNodeList) flags(cmd *cobra.Command) {
	flags := cmd.Flags()
	FlagColor(flags, &t.Color)
	FlagOutput(flags, &t.Output)
	FlagSort(flags, &t.Sort)
	flags.StringVar(&t.Name, "name", "", "the name of a network")
	FlagNodeSelectorFilter(flags, &t.NodeSelector)
}

// NewCmdNetworkNodeList returns the "network node list" command.
func NewCmdNetworkNodeList() *cobra.Command {
	var options CmdNetworkNodeList
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list what the last setup of each network made on each node",
		Long: "List what the last setup of each network made on each node, as the node recorded it: " +
			"the device it set up, the part of the network the node hands out, the peers it reaches, " +
			"and what failed. A network a node never set up is not listed for it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	options.flags(cmd)
	return cmd
}

// NewCmdNetworkPeerList returns the "network peer list" command.
func NewCmdNetworkPeerList() *cobra.Command {
	var options CmdNetworkPeerList
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list how each node reaches each other node of each network",
		Long: "List how each node reaches each other node of each network, as the last setup of the network " +
			"on the node made it: through a tunnel, named, or a route, from which address to which, and why " +
			"it could not. Each node records its own side: two nodes may reach each other differently.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	options.flags(cmd)
	return cmd
}

func (t *CmdNetworkNodeList) get() (*api.NetworkNodeList, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	params := api.GetNetworkNodesParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	if t.NodeSelector != "" {
		params.Node = &t.NodeSelector
	}
	resp, err := c.GetNetworkNodesWithResponse(context.Background(), &params)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return resp.JSON200, nil
	case 401:
		return nil, fmt.Errorf("%s", resp.JSON401)
	case 403:
		return nil, fmt.Errorf("%s", resp.JSON403)
	case 500:
		return nil, fmt.Errorf("%s", resp.JSON500)
	default:
		return nil, fmt.Errorf("unexpected status code %s", resp.Status())
	}
}

func (t *CmdNetworkNodeList) Run() error {
	data, err := t.get()
	if err != nil {
		return err
	}
	return output.Renderer{
		DefaultOutput: "tab=NAME:name,NODE:node,TYPE:type,DEV:dev,SUBNET:subnet,PEERS:peers[*].nodename,SKIPPED:skipped,ERROR:error,APPLIED_AT:applied_at",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// peerVia says how a node reaches a peer: through a tunnel, or a route.
func peerVia(p api.NetworkPeer) string {
	if p.Tunnel != "" {
		return "tunnel"
	}
	return "route"
}

func (t *CmdNetworkPeerList) Run() error {
	data, err := t.get()
	if err != nil {
		return err
	}
	l := make(unstructured.List, 0)
	for _, item := range data.Items {
		for _, p := range item.Peers {
			row := map[string]any{
				"name":   item.Name,
				"node":   item.Node,
				"peer":   p.Nodename,
				"via":    peerVia(p),
				"tunnel": p.Tunnel,
				"local":  p.Local,
				"remote": p.Remote,
				"subnet": p.Subnet,
				"error":  "",
			}
			if p.Error != nil {
				row["error"] = *p.Error
			}
			l = append(l, row)
		}
	}
	return output.Renderer{
		DefaultOutput: "tab=NAME:name,NODE:node,PEER:peer,VIA:via,TUNNEL:tunnel,LOCAL:local,REMOTE:remote,SUBNET:subnet,ERROR:error",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          l,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
