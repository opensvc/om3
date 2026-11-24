package commoncmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/danwakefield/fnmatch"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	CmdNodeWake struct {
		// Nodes is the selector of the cluster nodes to wake.
		Nodes string

		// Macs replace the wake#<node>.mac addresses.
		Macs []string

		// Local sends the magic packets from the local node, without
		// checking the heartbeat of the nodes to wake nor asking relays.
		Local bool

		// Wait for the woken nodes heartbeat to beat, up to Timeout.
		Wait    bool
		Timeout time.Duration
	}

	// wakeConfig is the wake#<node> section a client reads from the
	// cluster configuration of the api node.
	wakeConfig struct {
		hasMac bool
		relays []string
	}
)

// NewCmdNodeWake returns the node wake command. withLocal adds the --local
// flag, for the commands running on a cluster node.
func NewCmdNodeWake(withLocal bool) *cobra.Command {
	var options CmdNodeWake
	cmd := &cobra.Command{
		Use:   "wake",
		Short: "wake cluster nodes on lan",
		Long: "Wake the --node selected cluster nodes whose heartbeat does not beat, with Wake-on-LAN magic packets" +
			" sent by the relays up: the wake#<node>.relays nodes, or every peer up when not set." +
			" The nodes already awake are left as they are. A node just powered off is still seen awake until its heartbeat timeout expires.\n" +
			"The magic packets go to the wake#<node>.mac addresses, through the wake#<node>.broadcast addresses," +
			" or, when not set, through the broadcast address of the relay interface on the subnet of the address" +
			" the node name resolves to. A relay with no interface on that subnet sends nothing.\n" +
			"The nodes to wake must have Wake-on-LAN enabled in their firmware and network interface.",
		Example: "  om node wake --node n2\n" +
			"  om node wake --node n2,n3 --wait --timeout 10m\n" +
			"  om node wake --node n2 --mac 22:23:24:2d:01:12",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Nodes, "node", "", "the selector of the cluster nodes to wake")
	if err := cmd.MarkFlagRequired("node"); err != nil {
		panic(err)
	}
	flags.StringSliceVar(&options.Macs, "mac", nil, "the comma-separated mac addresses of the node to wake, replacing the wake#<node>.mac ones")
	flags.BoolVar(&options.Wait, "wait", false, "wait for the heartbeat of the woken nodes to beat")
	flags.DurationVar(&options.Timeout, "timeout", 5*time.Minute, "maximum duration to wait for the woken nodes heartbeat, with --wait")
	if withLocal {
		flags.BoolVar(&options.Local, "local", false, "send the magic packets from the local node, without checking the heartbeat of the nodes to wake")
	}
	return cmd
}

func (t *CmdNodeWake) Run() error {
	if t.Local {
		return t.runLocal()
	}
	return t.runRemote()
}

func (t *CmdNodeWake) runLocal() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	var errs error
	for _, nodename := range splitNodeSelector(t.Nodes) {
		if err := n.Wake(nodename, t.Macs); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, err))
		}
	}
	return errs
}

func (t *CmdNodeWake) runRemote() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	// The reader has to be opened before the cluster status is read: a node
	// becoming alive between the two would publish its NodeAlive unseen.
	var evReader event.ReadCloser
	if t.Wait {
		// No client timeout on the stream: it would cut it, the context
		// bounds it instead.
		streamClient, err := client.New(client.WithTimeout(0))
		if err != nil {
			return err
		}
		evReader, err = streamClient.NewGetEvents().
			SetRelatives(false).
			SetFilters([]string{"NodeAlive"}).
			SetDuration(t.Timeout).
			GetReader(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = evReader.Close() }()
	}

	b, err := c.NewGetClusterStatus().Get()
	if err != nil {
		return err
	}
	var data clusterdump.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	targets, err := expandClusterNodes(t.Nodes, data.Cluster.Config.Nodes)
	if err != nil {
		return err
	}

	var (
		errs    error
		pending []string
		start   = time.Now()
	)
	for _, target := range targets {
		if isReceiving(data, target) {
			_, _ = fmt.Fprintf(os.Stdout, "%s: already awake: its heartbeat beats\n", target)
			continue
		}
		relays, err := t.wakeOne(ctx, c, data, target)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", target, err))
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s: wake requested on %s\n", target, strings.Join(relays, ", "))
		pending = append(pending, target)
	}
	if t.Wait && len(pending) > 0 {
		errs = errors.Join(errs, waitNodesAlive(ctx, evReader, pending, start))
	}
	return errs
}

// wakeOne asks every relay up to wake target, and returns the relays that
// accepted.
func (t *CmdNodeWake) wakeOne(ctx context.Context, c *client.T, data clusterdump.Data, target string) ([]string, error) {
	cfg, err := getWakeConfig(ctx, c, target)
	if err != nil {
		return nil, err
	}
	if len(t.Macs) == 0 && !cfg.hasMac {
		return nil, fmt.Errorf("no mac address: set wake#%s.mac", target)
	}
	relays := wakeRelays(data, target, cfg.relays)
	if len(relays) == 0 {
		if len(cfg.relays) > 0 {
			return nil, fmt.Errorf("no wake#%s.relays node up (%s)", target, strings.Join(cfg.relays, ", "))
		}
		return nil, fmt.Errorf("no peer up to send the magic packet")
	}
	params := api.PostNodeActionWakeParams{Target: target}
	if len(t.Macs) > 0 {
		params.Mac = &t.Macs
	}
	sessionID := xsession.SessionID().UUID()
	params.SessionID = &sessionID

	var (
		accepted []string
		errs     error
	)
	for _, relay := range relays {
		if err := postNodeActionWake(ctx, c, relay, params); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		accepted = append(accepted, relay)
	}
	if len(accepted) == 0 {
		return nil, errs
	}
	if errs != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s: %s\n", target, errs)
	}
	return accepted, nil
}

func postNodeActionWake(ctx context.Context, c *client.T, relay string, params api.PostNodeActionWakeParams) error {
	response, err := c.PostNodeActionWakeWithResponse(ctx, relay, &params)
	if err != nil {
		return fmt.Errorf("relay %s: %w", relay, err)
	}
	switch {
	case response.JSON200 != nil:
		return nil
	case response.JSON400 != nil:
		return fmt.Errorf("relay %s: %s", relay, *response.JSON400)
	case response.JSON401 != nil:
		return fmt.Errorf("relay %s: %s", relay, *response.JSON401)
	case response.JSON403 != nil:
		return fmt.Errorf("relay %s: %s", relay, *response.JSON403)
	case response.JSON500 != nil:
		return fmt.Errorf("relay %s: %s", relay, *response.JSON500)
	default:
		return fmt.Errorf("relay %s: unexpected response: %s", relay, response.Status())
	}
}

// getWakeConfig reads the wake#<target> section of the cluster
// configuration. The option is a pattern, so an unset section answers no
// item instead of an error.
func getWakeConfig(ctx context.Context, c *client.T, target string) (wakeConfig, error) {
	var cfg wakeConfig
	kw := api.InQueryKeywords{"wake#" + target + ".*"}
	response, err := c.GetClusterConfigWithResponse(ctx, &api.GetClusterConfigParams{Kw: &kw})
	if err != nil {
		return cfg, err
	}
	switch {
	case response.JSON200 != nil:
	case response.JSON400 != nil:
		return cfg, fmt.Errorf("read wake#%s config: %s", target, *response.JSON400)
	case response.JSON401 != nil:
		return cfg, fmt.Errorf("read wake#%s config: %s", target, *response.JSON401)
	case response.JSON403 != nil:
		return cfg, fmt.Errorf("read wake#%s config: %s", target, *response.JSON403)
	case response.JSON500 != nil:
		return cfg, fmt.Errorf("read wake#%s config: %s", target, *response.JSON500)
	default:
		return cfg, fmt.Errorf("read wake#%s config: unexpected response: %s", target, response.Status())
	}
	for _, item := range response.JSON200.Items {
		option := item.Keyword[strings.LastIndex(item.Keyword, ".")+1:]
		switch option {
		case "mac":
			cfg.hasMac = strings.TrimSpace(item.Value) != ""
		case "relays":
			cfg.relays = splitNodeSelector(item.Value)
		}
	}
	return cfg, nil
}

// wakeRelays returns the relays allowed to wake target whose heartbeat
// beats: the configured ones, or every cluster node but target when none.
func wakeRelays(data clusterdump.Data, target string, configured []string) []string {
	candidates := configured
	if len(candidates) == 0 {
		candidates = data.Cluster.Config.Nodes
	}
	var relays []string
	for _, node := range candidates {
		if node == target || slices.Contains(relays, node) {
			continue
		}
		if isReceiving(data, node) {
			relays = append(relays, node)
		}
	}
	return relays
}

// splitNodeSelector splits a node selector on commas and spaces.
func splitNodeSelector(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

// expandClusterNodes resolves the node selector against the cluster nodes:
// names and fnmatch patterns. A down node is out of the nodes info the
// node selectors read, so they can not expand it.
func expandClusterNodes(selector string, nodes []string) ([]string, error) {
	var l []string
	for _, s := range splitNodeSelector(strings.ToLower(selector)) {
		if strings.ContainsAny(s, "*?[") {
			for _, node := range nodes {
				if fnmatch.Match(s, node, fnmatch.FNM_IGNORECASE|fnmatch.FNM_PATHNAME) && !slices.Contains(l, node) {
					l = append(l, node)
				}
			}
			continue
		}
		if !slices.Contains(nodes, s) {
			return nil, fmt.Errorf("node %s is not a cluster node", s)
		}
		if !slices.Contains(l, s) {
			l = append(l, s)
		}
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("no cluster node matches %q", selector)
	}
	return l, nil
}

// waitNodesAlive waits for the NodeAlive events of the nodes.
func waitNodesAlive(ctx context.Context, evReader event.ReadCloser, nodes []string, start time.Time) error {
	pending := slices.Clone(nodes)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%s: heartbeat not beating yet: %w", strings.Join(pending, ", "), err)
		}
		ev, err := evReader.Read()
		if err != nil {
			return fmt.Errorf("%s: heartbeat not beating yet: %w", strings.Join(pending, ", "), err)
		}
		msg, err := msgbus.EventToMessage(*ev)
		if err != nil {
			return err
		}
		m, ok := msg.(*msgbus.NodeAlive)
		if !ok {
			continue
		}
		// The node label of this event is the node that observes the
		// heartbeat, not the beating one: match the payload.
		if i := slices.Index(pending, m.Node); i >= 0 {
			_, _ = fmt.Fprintf(os.Stdout, "%s: awake after %s\n", m.Node, time.Since(start).Round(time.Second))
			pending = slices.Delete(pending, i, i+1)
		}
	}
	return nil
}

// isReceiving returns whether the api node of data receives the heartbeat
// of node on any of its rx streams. The api node itself is awake.
//
// The tx streams are not read: a unicast tx is beating while its writes
// succeed, and the writes to a connection established to a peer powered
// off succeed until the kernel gives up retransmitting, many minutes later.
func isReceiving(data clusterdump.Data, node string) bool {
	apiNode := data.Daemon.Nodename
	if node == apiNode {
		return true
	}
	for _, stream := range data.Cluster.Node[apiNode].Daemon.Heartbeat.Streams {
		if strings.HasSuffix(stream.ID, ".rx") && stream.Peers[node].IsBeating {
			return true
		}
	}
	return false
}
