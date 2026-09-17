package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	CmdClusterEnroll struct {
		// Node is the location of the node to enroll, in the
		// [<scheme>://]<addr>[:<port>] format.
		Node string

		// TokenFile is the path of a file holding an access token with the
		// join role, created on the node to enroll. When empty, the
		// OSVC_JOIN_TOKEN environment variable is used.
		TokenFile string

		// JoinAddr is the location the enrolled node must use to reach the
		// cluster it joins.
		JoinAddr string

		// Timeout is the lifetime of the join token handed to the enrolled
		// node, and the maximum duration to wait for the join to complete.
		Timeout time.Duration

		// Wait blocks until the enrolled node heartbeat is beating.
		Wait bool
	}
)

var (
	ErrCmdClusterEnroll = errors.New("command cluster enroll")
)

func NewCmdClusterEnroll() *cobra.Command {
	var options CmdClusterEnroll
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "make a foreign node leave its cluster and join this one",
		Long: "Order a node to leave its cluster and join the cluster of the api node.\n" +
			"The node to enroll must be a single node cluster: a node that still has peers is refused," +
			" because nothing tells them to drop it from their cluster.nodes.\n" +
			"The '--token' names a file holding an access token with the join role, created on the node to" +
			" enroll by the 'om daemon auth --role join' command. Its 'ca' claim is used to trust that node" +
			" certificate, and only a token carrying that role holds the claim.\n" +
			"The node is drained before it leaves its cluster. As a single node cluster has nowhere to" +
			" relocate its instances, they are stopped, stay down, and removed from config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Node, "node", "", "the location of the node to enroll, in the [<scheme>://]<addr>[:<port>] format")
	if err := cmd.MarkFlagRequired("node"); err != nil {
		panic(err)
	}
	FlagToken(flags, &options.TokenFile)
	flags.StringVar(&options.JoinAddr, "join-addr", "", "the location the enrolled node must use to reach this cluster,"+
		" in the [<scheme>://]<addr>[:<port>] format."+
		" It is refused when the cluster certificate is not valid for its host, because the enrolled node"+
		" would fail to verify it."+
		" Defaults to a name that certificate is valid for")
	flags.DurationVar(&options.Timeout, "timeout", time.Hour, "maximum duration to wait for the enrolled node to join."+
		" It is also the lifetime of the join token, so it must outlive the node drain")
	flags.BoolVar(&options.Wait, "wait", false, "wait for the enrolled node heartbeat to beat in this cluster")
	return cmd
}

func (t *CmdClusterEnroll) Run() error {
	if err := t.run(); err != nil {
		return fmt.Errorf("%w: %w", ErrCmdClusterEnroll, err)
	}
	return nil
}

func (t *CmdClusterEnroll) run() error {
	token, err := t.token()
	if err != nil {
		return err
	}
	c, err := client.New()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	// The reader has to be opened before the post: JoinSuccess is published as
	// soon as the api node has updated its cluster.nodes, which can happen
	// before we get the response. The nodename the events carry is only known
	// from that response, so the filters cannot be qualified yet: ask for the
	// kinds and match the node client side.
	var evReader event.ReadCloser
	if t.Wait {
		evReader, err = c.NewGetEvents().
			SetRelatives(false).
			SetFilters([]string{"JoinSuccess", "JoinError", "JoinIgnored", "NodeAlive"}).
			SetDuration(t.Timeout).
			GetReader(ctx)
		if err != nil {
			return err
		}
		defer func() {
			_ = evReader.Close()
		}()
	}

	timeout := t.Timeout.String()
	body := api.PostClusterEnrollJSONRequestBody{
		Node:    t.Node,
		Token:   token,
		Timeout: &timeout,
	}
	if t.JoinAddr != "" {
		body.JoinAddr = &t.JoinAddr
	}
	resp, err := c.PostClusterEnrollWithResponse(ctx, body)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusBadRequest:
		return fmt.Errorf("%s", resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s", resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s", resp.JSON403)
	case http.StatusConflict:
		return fmt.Errorf("%s", resp.JSON409)
	case http.StatusBadGateway:
		return fmt.Errorf("%s", resp.JSON502)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode(), strings.TrimSpace(string(resp.Body)))
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected empty response body")
	}
	node := resp.JSON200.Node
	_, _ = fmt.Fprintf(os.Stdout, "Node %s accepted the join order\n", node)

	if !t.Wait {
		_, _ = fmt.Fprintf(os.Stdout, "Follow with: om daemon events --filter 'NodeAlive,.node=%s'\n", node)
		return nil
	}
	return t.waitResult(ctx, evReader, node)
}

// token returns the token named by --token, or the one the environment holds.
func (t *CmdClusterEnroll) token() (string, error) {
	token, err := SecretFromFileOrEnv(t.TokenFile, env.JoinTokenVar)
	if err != nil {
		return "", fmt.Errorf("%w: --token: %w", ErrFlagInvalid, err)
	}
	if token == "" {
		return "", fmt.Errorf("%w: token is empty: use --token or env %s", ErrFlagInvalid, env.JoinTokenVar)
	}
	return token, nil
}

// waitResult reports the join progress of the node until its heartbeat beats.
//
// JoinSuccess only tells the api node updated its cluster.nodes: the enrolled
// node has not drained, stopped, reconfigured, or restarted yet. NodeAlive is
// what proves it is a live member of this cluster.
func (t *CmdClusterEnroll) waitResult(ctx context.Context, evReader event.ReadCloser, node string) error {
	joined := false
	for {
		select {
		case <-ctx.Done():
			if joined {
				return fmt.Errorf("node %s was added to the cluster nodes but its heartbeat is not beating yet: %w", node, ctx.Err())
			}
			return ctx.Err()
		default:
		}
		ev, err := evReader.Read()
		if err != nil {
			if joined {
				return fmt.Errorf("node %s was added to the cluster nodes but its heartbeat is not beating yet: %w", node, err)
			}
			return err
		}
		msg, err := msgbus.EventToMessage(*ev)
		if err != nil {
			return err
		}
		switch m := msg.(type) {
		case *msgbus.JoinSuccess:
			if m.AddedNode != node {
				continue
			}
			joined = true
			_, _ = fmt.Fprintf(os.Stdout, "Cluster nodes updated\n")
			_, _ = fmt.Fprintf(os.Stdout, "Waiting for %s to beat\n", node)
		case *msgbus.JoinIgnored:
			if m.CandidateNode != node {
				continue
			}
			// Already a cluster node: it may be beating already, and a beating
			// node publishes no new NodeAlive, so don't wait for one.
			_, _ = fmt.Fprintf(os.Stdout, "Join ignored: %s is already a cluster node\n", node)
			return nil
		case *msgbus.JoinError:
			if m.CandidateNode != node {
				continue
			}
			return fmt.Errorf("join error for node %s: %s", node, m.Reason)
		case *msgbus.NodeAlive:
			// The node label of this event is the node that observes the
			// heartbeat, not the beating one: match the payload.
			if m.Node != node {
				continue
			}
			_, _ = fmt.Fprintf(os.Stdout, "Node %s is alive\n", node)
			return nil
		}
	}
}
