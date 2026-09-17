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
	"github.com/opensvc/om3/v3/core/credential"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	CmdClusterEvict struct {
		// Node is the nodename to remove from the cluster nodes.
		Node string

		// CredentialFile is the path of a file holding the
		// <username>:<password> of the user to create on the evicted node.
		CredentialFile string

		// Timeout is the maximum duration of the leave forked on the evicted
		// node, and the maximum duration to wait for it when Wait is set.
		Timeout time.Duration

		// Wait blocks until the cluster has dropped the evicted node.
		Wait bool
	}
)

var (
	ErrCmdClusterEvict = errors.New("command cluster evict")
)

func NewCmdClusterEvict() *cobra.Command {
	var options CmdClusterEvict
	cmd := &cobra.Command{
		Use:   "evict",
		Short: "make a cluster node leave this cluster",
		Long: "Remove a node from the cluster nodes.\n" +
			"The node to evict is ordered to leave: it asks a peer to drop it from the cluster nodes," +
			" then restarts its daemon alone in a new single node cluster.\n" +
			"The node to evict must be drained, else what it runs would stay up on a node the cluster no" +
			" longer knows about, free to start a second time elsewhere. Drain it with 'om node drain --node <node> --wait'.\n" +
			"The evicted node ends up with a cluster secret of its own, so no user, token or certificate of" +
			" this cluster can reach its api anymore. Use '--credential' to have a user created to reach it with.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Node, "node", "", "the name of the cluster node to evict")
	if err := cmd.MarkFlagRequired("node"); err != nil {
		panic(err)
	}
	FlagCredential(flags, &options.CredentialFile)
	flags.DurationVar(&options.Timeout, "timeout", time.Hour, "maximum duration of the leave forked on the evicted node."+
		" It is also the maximum duration to wait for it when --wait is set")
	flags.BoolVar(&options.Wait, "wait", false, "wait for the cluster nodes to be updated")
	return cmd
}

func (t *CmdClusterEvict) Run() error {
	if err := t.run(); err != nil {
		return fmt.Errorf("%w: %w", ErrCmdClusterEvict, err)
	}
	return nil
}

func (t *CmdClusterEvict) run() error {
	cred, err := t.credential()
	if err != nil {
		return err
	}
	c, err := client.New()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	// The reader has to be opened before the post: the peer the evicted node
	// asks can update its cluster.nodes, and our api node can learn about it,
	// before we get the response.
	var evReader event.ReadCloser
	if t.Wait {
		filters := []string{
			"LeaveSuccess,removed_node=" + t.Node,
			"LeaveError,candidate_node=" + t.Node,
			"LeaveIgnored,candidate_node=" + t.Node,
		}
		evReader, err = c.NewGetEvents().
			SetRelatives(false).
			SetFilters(filters).
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
	body := api.PostClusterEvictJSONRequestBody{
		Nodename: t.Node,
		Timeout:  &timeout,
	}
	if cred != "" {
		body.Credential = &cred
	}
	resp, err := c.PostClusterEvictWithResponse(ctx, body)
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
	_, _ = fmt.Fprintf(os.Stdout, "Node %s accepted the leave order\n", t.Node)

	if !t.Wait {
		_, _ = fmt.Fprintf(os.Stdout, "Follow with: om daemon events --filter 'LeaveSuccess,removed_node=%s'\n", t.Node)
		return nil
	}
	return t.waitResult(ctx, evReader)
}

// credential returns the credential to hand to the evicted node, and an empty
// string when none was given.
//
// It is parsed here as well as in the handler, so that a malformed one is
// reported before anything is ordered.
func (t *CmdClusterEvict) credential() (string, error) {
	s, err := SecretFromFileOrEnv(t.CredentialFile, env.CredentialVar)
	if err != nil {
		return "", fmt.Errorf("%w: --credential: %w", ErrFlagInvalid, err)
	}
	if s == "" {
		return "", nil
	}
	if _, _, err := credential.Parse(s); err != nil {
		return "", fmt.Errorf("%w: %w", ErrFlagInvalid, err)
	}
	return s, nil
}

// waitResult reports the leave progress until the cluster nodes are updated.
//
// LeaveSuccess only says this cluster dropped the node. The daemon restart and
// the user creation happen afterwards, on a node this cluster no longer
// observes, so there is nothing more to wait for here.
func (t *CmdClusterEvict) waitResult(ctx context.Context, evReader event.ReadCloser) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ev, err := evReader.Read()
		if err != nil {
			return err
		}
		msg, err := msgbus.EventToMessage(*ev)
		if err != nil {
			return err
		}
		switch m := msg.(type) {
		case *msgbus.LeaveSuccess:
			_, _ = fmt.Fprintf(os.Stdout, "Cluster nodes updated\n")
			_, _ = fmt.Fprintf(os.Stdout, "Node %s is restarting alone, its instances stay down\n", m.RemovedNode)
			return nil
		case *msgbus.LeaveIgnored:
			_, _ = fmt.Fprintf(os.Stdout, "Leave ignored: %s is not a cluster node\n", m.CandidateNode)
			return nil
		case *msgbus.LeaveError:
			return fmt.Errorf("leave error for node %s: %s", m.CandidateNode, m.Reason)
		}
	}
}
