package omcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/credential"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	CmdClusterLeave struct {
		CmdDaemonCommon

		// Timeout is the maximum duration for leave
		Timeout time.Duration

		// APINode is a cluster node where the leave request will be posted
		APINode string

		// CredentialFile is the path of a file holding the
		// <username>:<password> of the user to create once the daemon has
		// restarted alone. When empty, the OSVC_CREDENTIAL environment
		// variable is used. Without either, no user is created.
		CredentialFile string

		peerClient *client.T
		localhost  string
		evReader   event.ReadCloser
	}
)

var (
	ErrCmdClusterLeave = errors.New("command cluster leave")
)

func (t *CmdClusterLeave) Run() error {
	err := t.run()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCmdClusterLeave, err)
	}
	return nil
}

func (t *CmdClusterLeave) run() (err error) {
	var (
		tk string

		localClient *client.T

		deadLine time.Time
	)
	t.localhost = hostname.Hostname()

	// Resolve the credential before draining anything. Once the leave is
	// under way this node is no longer reachable with the credentials of the
	// cluster it leaves, so a credential refused at that point would leave it
	// with none at all.
	username, password, err := t.credential()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if t.Timeout > 0 {
		deadLine = time.Now().Add(t.Timeout)
		ctxWithDeadline, deadlineCancel := context.WithDeadline(ctx, deadLine)
		defer deadlineCancel()
		ctx = ctxWithDeadline
	}

	if t.APINode, err = t.peerClusterNode(); err != nil {
		return fmt.Errorf("unable to find a peer node to announce we are leaving: %w", err)
	}

	if t.isRunning() {
		if err := t.nodeDrain(ctx); err != nil {
			return err
		}
	}

	if localClient, err = client.New(); err != nil {
		return fmt.Errorf("unable to create leave token: %w", err)
	} else {
		// the default token duration should be enough for next steps: post leave and wait for completion
		params := api.PostAuthTokenParams{Role: &api.Roles{api.Leave}}
		if deadLine, hasDeadline := ctx.Deadline(); hasDeadline {
			// ensure token duration can be used until deadline reached
			tkDuration := deadLine.Sub(time.Now()).String()
			params.AccessDuration = &tkDuration
		}
		resp, err := localClient.PostAuthTokenWithResponse(ctx, &params)
		if err != nil {
			return fmt.Errorf("can't get leave token: %w", err)
		} else if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("can't get leave token: got %d wanted %d", resp.StatusCode(), http.StatusOK)
		} else {
			tk = resp.JSON200.AccessToken
		}
	}

	t.peerClient, err = client.New(
		client.WithURL(daemonenv.HTTPNodeURL(t.APINode)),
		client.WithBearer(tk),
	)
	if err != nil {
		return
	}

	if err := t.setEvReader(ctx, deadLine.Sub(time.Now())); err != nil {
		return err
	}
	defer func() {
		_ = t.evReader.Close()
	}()

	if err := t.leave(ctx, t.peerClient); err != nil {
		return err
	}
	if err := t.waitResult(ctx); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Stop daemon\n")
	if err := (&CmdDaemonStop{}).Run(); err != nil {
		return err
	}

	if err := t.backupLocalConfig(".pre-cluster-leave-etc"); err != nil {
		return err
	}

	if err := t.cleanupVarDir(); err != nil {
		return fmt.Errorf("cleanup opensvc var dir: %w", err)
	}

	if err := t.cleanupAndMandatoryDirectories(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Start daemon\n")
	if err := (&CmdDaemonStart{}).Run(); err != nil {
		return err
	}

	if username != "" {
		if err := t.createUser(ctx, username, password); err != nil {
			return fmt.Errorf("create user %s: %w", username, err)
		}
	}
	return nil
}

// credential returns the username and password of the user to create once we
// are alone, and two empty strings when no credential was given.
func (t *CmdClusterLeave) credential() (string, string, error) {
	s, err := commoncmd.SecretFromFileOrEnv(t.CredentialFile, env.CredentialVar)
	if err != nil {
		return "", "", fmt.Errorf("%w: --credential: %w", commoncmd.ErrFlagInvalid, err)
	}
	if s == "" {
		return "", "", nil
	}
	username, password, err := credential.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", commoncmd.ErrFlagInvalid, err)
	}
	return username, password, nil
}

// createUser creates the usr object the operator reaches our api with, now
// that we are a cluster of our own.
//
// The cluster we left holds no authority here anymore: our new cluster has its
// own name and secret, so none of its users, tokens or certificates work on
// us. Without this object, the api is only reachable through the unix socket,
// from a root shell on this node.
func (t *CmdClusterLeave) createUser(ctx context.Context, username, password string) error {
	// The daemon has just been started, so the configuration it bootstraps is
	// due immediately. Bound the wait on its own, so that a leave run without
	// --timeout does not hang here forever.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if err := t.waitClusterConfig(ctx); err != nil {
		return err
	}
	p := naming.Path{Name: username, Namespace: naming.NsSys, Kind: naming.KindUsr}
	_, _ = fmt.Fprintf(os.Stdout, "Create %s\n", p)

	usr, err := object.NewUsr(p)
	if err != nil {
		return err
	}
	ops := []keyop.T{
		{Key: key.Parse("id"), Op: keyop.Set, Value: uuid.New().String()},
		{Key: key.Parse("grant"), Op: keyop.Set, Value: string(rbac.GrantRoot)},
	}
	if err := usr.Config().Set(ops...); err != nil {
		return err
	}
	// ChangeOrAdd, not Add: a password left over from a previous leave with
	// the same username must be the one the operator was just handed.
	if err := usr.TransactionChangeOrAddKey("password", []byte(password)); err != nil {
		return err
	}
	return usr.Config().Commit()
}

// waitClusterConfig reloads the cluster configuration the restarted daemon
// bootstrapped, and waits for it to carry the name and secret the usr object
// is encrypted with.
//
// The configuration this process read at startup was the one of the cluster we
// left, and the directory holding it has been moved aside since. Encrypting
// the password with that stale secret would produce a value the daemon can not
// decode.
func (t *CmdClusterLeave) waitClusterConfig(ctx context.Context) error {
	const interval = 500 * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastErr error
	for {
		cfg, err := object.SetClusterConfig()
		switch {
		case err != nil:
			lastErr = err
		case cfg.Name == "":
			lastErr = fmt.Errorf("cluster name is empty")
		case cfg.Secret() == "":
			lastErr = fmt.Errorf("cluster secret is empty")
		default:
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for the new cluster config: %w: %w", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func (t *CmdClusterLeave) setEvReader(ctx context.Context, duration time.Duration) (err error) {
	filters := []string{
		"LeaveSuccess,removed_node=" + t.localhost,
		"LeaveError,candidate_node=" + t.localhost,
		"LeaveIgnored,candidate_node=" + t.localhost,
	}

	getEvents := t.peerClient.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters)

	if duration > 0 {
		getEvents.SetDuration(duration)
	}

	t.evReader, err = getEvents.GetReader(ctx)
	return
}

func (t *CmdClusterLeave) waitResult(ctx context.Context) error {
	for {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			ev, err := t.evReader.Read()
			if err != nil {
				return err
			}
			switch ev.Kind {
			case (&msgbus.LeaveSuccess{}).Kind():
				_, _ = fmt.Fprintf(os.Stdout, "Cluster nodes updated\n")
				return nil
			case (&msgbus.LeaveError{}).Kind():
				err := fmt.Errorf("leave error: %s", ev.Data)
				return err
			case (&msgbus.LeaveIgnored{}).Kind():
				// TODO parse Reason
				_, _ = fmt.Fprintf(os.Stdout, "Leave ignored: %s", ev.Data)
				return nil
			default:
				return fmt.Errorf("unexpected event %s %v", ev.Kind, ev.Data)
			}
		}
	}
}

func (t *CmdClusterLeave) leave(ctx context.Context, c *client.T) error {
	_, _ = fmt.Fprintf(os.Stdout, "Daemon leave\n")
	params := api.PostClusterLeaveParams{
		Node: t.localhost,
	}
	if resp, err := c.PostClusterLeave(ctx, &params); err != nil {
		return fmt.Errorf("post cluster leave error: %w", err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post cluster leave unexpected status code %s", resp.Status)
	}
	return nil
}

func (t *CmdClusterLeave) peerClusterNode() (string, error) {
	if ccfg, err := object.NewCluster(object.WithVolatile(true)); err != nil {
		return "", err
	} else if clusterNodes, err := ccfg.Nodes(); err != nil {
		return "", err
	} else if len(clusterNodes) == 0 {
		return "", fmt.Errorf("unexpected cluster nodes: %v", clusterNodes)
	} else if len(clusterNodes) == 1 {
		return "", fmt.Errorf("not available on single node cluster")
	} else {
		for _, node := range clusterNodes {
			if node != "" && node != hostname.Hostname() {
				return node, nil
			}
		}
		return "", fmt.Errorf("unexpected cluster nodes: %v", clusterNodes)
	}
}
