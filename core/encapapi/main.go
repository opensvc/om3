// Package encapapi drives the instance of an object on a guest through the
// api of the guest daemon, for the master of the object.
package encapapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/actionlogs"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	// T drives the instance of the object path on the node of a guest.
	T struct {
		c    *client.T
		node string
		path naming.Path
	}

	// ActionOptions are the options of an action, the ones the master gives
	// its encap instances.
	ActionOptions struct {
		RID             string
		Leader          bool
		DisableRollback bool
		Force           bool
	}

	// accepted is what a post of an action answers.
	accepted struct {
		code int
		exec uuid.UUID
		body []byte
	}

	// execEnd is what the end of an exec event says of it.
	execEnd struct {
		ExecID   uuid.UUID `json:"exec_id"`
		Error    string    `json:"error"`
		ExitCode int       `json:"exit_code"`
	}

	// instanceItem is the part of the instance route read here.
	instanceItem struct {
		Data struct {
			Config  instance.Config  `json:"config"`
			Monitor instance.Monitor `json:"monitor"`
			Status  instance.Status  `json:"status"`
		} `json:"data"`
	}
)

// ErrNotFound says the guest has no such object.
var ErrNotFound = errors.New("object not found")

// New returns the driver of the instance of path on node, through c.
func New(c *client.T, node string, path naming.Path) *T {
	return &T{c: c, node: node, path: path}
}

// Ping says whether the guest daemon answers as a root peer. Nothing has been
// asked of the guest when it fails, so the caller can reach it another way.
func (t *T) Ping(ctx context.Context) error {
	resp, err := t.c.GetAuthWhoAmIWithResponse(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("guest daemon: %s", resp.Status())
	}
	return nil
}

// HasConfig says whether the guest has the object.
func (t *T) HasConfig(ctx context.Context) (bool, error) {
	resp, err := t.c.GetObjectConfigFileWithResponse(ctx, t.path.Namespace, t.path.Kind, t.path.Name, nil)
	if err != nil {
		return false, err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("%s: read the config: %s: %s", t.path, resp.Status(), resp.Body)
	}
}

// InstallConfig writes the configuration b on the guest, creating the object
// or replacing the configuration it has, with its own id kept. It returns
// once the guest daemon knows of it.
func (t *T) InstallConfig(ctx context.Context, b []byte) error {
	restore := true
	waitLocal := true
	force := true
	params := api.PostObjectConfigFileParams{Restore: &restore, WaitLocal: &waitLocal, Force: &force}
	resp, err := t.c.PostObjectConfigFileWithBodyWithResponse(ctx, t.path.Namespace, t.path.Kind, t.path.Name, &params, "application/octet-stream", strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("%s: install the config: %s: %s", t.path, resp.Status(), resp.Body)
	}
	return nil
}

// Status refreshes the status of the instance, and returns its states. It
// returns ErrNotFound when the guest has no such object.
func (t *T) Status(ctx context.Context) (*instance.States, error) {
	session := xsession.SessionID().UUID()
	err := t.run(ctx, session, nil, func() (accepted, error) {
		params := api.PostInstanceActionStatusParams{SessionID: &session}
		resp, err := t.c.PostInstanceActionStatusWithResponse(ctx, t.node, t.path.Namespace, t.path.Kind, t.path.Name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	})
	if err != nil {
		return nil, err
	}
	resp, err := t.c.GetInstanceWithResponse(ctx, t.node, t.path.Namespace, t.path.Kind, t.path.Name)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		return t.parseStates(resp.Body)
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("%s: read the instance: %s: %s", t.path, resp.Status(), resp.Body)
	}
}

func (t *T) parseStates(b []byte) (*instance.States, error) {
	var item instanceItem
	if err := json.Unmarshal(b, &item); err != nil {
		return nil, fmt.Errorf("%s: read the instance: %w", t.path, err)
	}
	return &instance.States{
		Path:    t.path,
		Node:    instance.Node{Name: t.node},
		Config:  item.Data.Config,
		Status:  item.Data.Status,
		Monitor: item.Data.Monitor,
	}, nil
}

// Supports says whether action can be asked through the api. The other ones
// are for the caller to ask the guest another way.
func (t *T) Supports(action string) bool {
	switch action {
	case "provision", "shutdown", "start", "startstandby", "stop", "unprovision":
		return true
	default:
		return false
	}
}

// Action runs action on the instance and returns when it ended, with the
// error of the exec when it failed. Its logs are written to w as they come.
func (t *T) Action(ctx context.Context, action string, o ActionOptions, w io.Writer) error {
	session := xsession.SessionID().UUID()
	return t.run(ctx, session, w, func() (accepted, error) {
		return t.post(ctx, action, session, o)
	})
}

func (t *T) post(ctx context.Context, action string, session uuid.UUID, o ActionOptions) (accepted, error) {
	var (
		ns, kind, name = t.path.Namespace, t.path.Kind, t.path.Name
		rid            *string
		leader         *bool
		rollback       *bool
		force          *bool
	)
	if o.RID != "" {
		rid = &o.RID
	}
	if o.Leader {
		leader = &o.Leader
	}
	if o.DisableRollback {
		rollback = &o.DisableRollback
	}
	if o.Force {
		force = &o.Force
	}
	switch action {
	case "provision":
		params := api.PostInstanceActionProvisionParams{SessionID: &session, Rid: rid, Leader: leader, DisableRollback: rollback, Force: force}
		resp, err := t.c.PostInstanceActionProvisionWithResponse(ctx, t.node, ns, kind, name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	case "shutdown":
		params := api.PostInstanceActionShutdownParams{SessionID: &session, Rid: rid, Force: force}
		resp, err := t.c.PostInstanceActionShutdownWithResponse(ctx, t.node, ns, kind, name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	case "start":
		params := api.PostInstanceActionStartParams{SessionID: &session, Rid: rid, DisableRollback: rollback, Force: force}
		resp, err := t.c.PostInstanceActionStartWithResponse(ctx, t.node, ns, kind, name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	case "startstandby":
		params := api.PostInstanceActionStartStandbyParams{SessionID: &session, Rid: rid, DisableRollback: rollback, Force: force}
		resp, err := t.c.PostInstanceActionStartStandbyWithResponse(ctx, t.node, ns, kind, name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	case "stop":
		params := api.PostInstanceActionStopParams{SessionID: &session, Rid: rid, Force: force}
		resp, err := t.c.PostInstanceActionStopWithResponse(ctx, t.node, ns, kind, name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	case "unprovision":
		params := api.PostInstanceActionUnprovisionParams{SessionID: &session, Rid: rid, Leader: leader, Force: force}
		resp, err := t.c.PostInstanceActionUnprovisionWithResponse(ctx, t.node, ns, kind, name, &params)
		if err != nil {
			return accepted{}, err
		}
		return newAccepted(resp.StatusCode(), resp.JSON200, resp.Body), nil
	default:
		return accepted{}, fmt.Errorf("%s: action %s has no route", t.path, action)
	}
}

func newAccepted(code int, v *api.InstanceActionAccepted, body []byte) accepted {
	a := accepted{code: code, body: body}
	if v != nil {
		a.exec = v.ExecID
	}
	return a
}

// run posts an action and waits for its exec to end. The event stream, and
// the log stream when w is set, open first, so neither misses an action
// quicker than the answer to its post. The session may be shared by several
// execs, so the end waited for is the one of the exec the post names.
func (t *T) run(ctx context.Context, session uuid.UUID, w io.Writer, post func() (accepted, error)) error {
	filters := []string{
		fmt.Sprintf("ExecFailed,path=%s,.session_id=\"%s\"", t.path, session),
		fmt.Sprintf("ExecSuccess,path=%s,.session_id=\"%s\"", t.path, session),
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, err := t.c.NewGetEvents().SetFilters(filters).SetNodename(t.node).GetReader(ctx)
	if err != nil {
		return fmt.Errorf("%s: open the event stream: %w", t.path, err)
	}
	defer func() { _ = reader.Close() }()
	if w != nil {
		logs, err := actionlogs.StartSessionLogsTo(t.c, []string{t.node}, session, "auto", w)
		if err != nil {
			return fmt.Errorf("%s: open the log stream: %w", t.path, err)
		}
		defer logs.Stop()
	}
	a, err := post()
	if err != nil {
		return err
	}
	switch a.code {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("%s: %d: %s", t.path, a.code, a.body)
	}
	return waitExec(reader, a.exec)
}

// waitExec reads the events until the one of the end of exec, and returns the
// error it carries, ErrNotFound when the exec found no such object: the daemon
// accepts an action on an object it still knows of or never checked, and the
// command is what tells.
func waitExec(reader event.Reader, exec uuid.UUID) error {
	for {
		ev, err := reader.Read()
		if err != nil {
			return fmt.Errorf("wait for the end of exec %s: %w", exec, err)
		}
		var end execEnd
		if err := json.Unmarshal(ev.Data, &end); err != nil {
			return fmt.Errorf("wait for the end of exec %s: %w", exec, err)
		}
		if end.ExecID != exec {
			continue
		}
		if ev.Kind == "ExecFailed" {
			if end.ExitCode == xerrors.ExitCodeObjectNotFound {
				return ErrNotFound
			}
			return errors.New(end.Error)
		}
		return nil
	}
}
