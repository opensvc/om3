package object

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/httphelper"
)

type (
	// QueuedAction defines the model for QueuedAction.
	QueuedAction struct {
		Command    string    `json:"command"`
		DequeuedAt time.Time `json:"dequeued_at"`
		Id         int       `json:"id"`
		Nodename   string    `json:"nodename"`
		QueuedAt   time.Time `json:"queued_at"`
		Status     string    `json:"status"`
		SvcId      string    `json:"svc_id"`
		Svcname    string    `json:"svcname"`
	}

	// QueuedActionDone defines the action queued result processed by the node
	QueuedActionDone struct {
		DequeuedAt time.Time `json:"dequeued_at"`
		Id         int       `json:"id"`
		Ret        int       `json:"ret"`
		Stderr     string    `json:"stderr"`
		Stdout     string    `json:"stdout"`
	}

	// QueuedActionRunning defines the model for QueuedActionRunning.
	QueuedActionRunning struct {
		// Ids list of queued action ids
		Ids []int `json:"ids"`
	}

	// QueuedActions defines the model for QueuedActions.
	QueuedActions struct {
		Actions []QueuedAction `json:"actions"`
	}

	oc3dequeue struct {
		oc3 *httphelper.T
	}
)

func (t *Node) Dequeue() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info().Msg("fetch actions from collector")

	oc3, err := t.CollectorFeeder()
	if err != nil {
		return err
	}

	runner := &oc3dequeue{oc3: oc3}

	queuedActions, err := runner.getActions(ctx)
	if err != nil {
		return fmt.Errorf("dequeue actions: %w", err)
	}
	log.Info().Msgf("fetch %d actions from collector", len(queuedActions))

	if len(queuedActions) == 0 {
		return nil
	}

	ids := make([]int, 0, len(queuedActions))
	for _, a := range queuedActions {
		ids = append(ids, a.Id)
	}
	if err := runner.ackReceive(ctx, ids); err != nil {
		return fmt.Errorf("dequeue action running: %w", err)
	}

	for _, action := range queuedActions {
		done, err1 := action.exec(ctx)
		if err1 != nil {
			log.Error().Msgf("dequeue action %d: %d", action.Id, done.Ret)
		} else {
			log.Info().Msgf("dequeue action %d: %d", action.Id, done.Ret)
		}
		err = errors.Join(err, err1)

		err2 := runner.sendDone(ctx, done)
		err = errors.Join(err, err2)
	}

	return err
}

func (t *oc3dequeue) ackReceive(ctx context.Context, ids []int) error {
	var (
		req  *http.Request
		resp *http.Response
		err  error

		ioReader io.Reader

		method = http.MethodPost
		path   = oc3path.FeedNodeActionQRunning
	)

	data := &QueuedActionRunning{
		Ids: ids,
	}
	if b, err := json.Marshal(data); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	req, err = t.oc3.NewRequestWithContext(ctx, method, path, ioReader)
	if err != nil {
		return fmt.Errorf("create request %s %s: %w", method, path, err)
	}

	resp, err = t.oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return CollectorResponseStatusCheck(resp, method, path, []int{http.StatusAccepted})
}

func (t *oc3dequeue) sendDone(ctx context.Context, result QueuedActionDone) error {
	var (
		req  *http.Request
		resp *http.Response
		err  error

		ioReader io.Reader

		method = http.MethodPost
		path   = oc3path.FeedNodeActionQDone
	)

	if b, err := json.Marshal(result); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	req, err = t.oc3.NewRequestWithContext(ctx, method, path, ioReader)
	if err != nil {
		return fmt.Errorf("create request %s %s: %w", method, path, err)
	}

	resp, err = t.oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return CollectorResponseStatusCheck(resp, method, path, []int{http.StatusAccepted})
}

func (t *oc3dequeue) getActions(ctx context.Context) ([]QueuedAction, error) {
	var (
		req  *http.Request
		resp *http.Response
		err  error

		method = http.MethodGet
		path   = oc3path.FeedNodeActionQ
	)

	req, err = t.oc3.NewRequestWithContext(ctx, method, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request %s %s: %w", method, path, err)
	}

	resp, err = t.oc3.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := CollectorResponseStatusCheck(resp, method, path, []int{http.StatusOK}); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(resp.Body)
	data := &QueuedActions{}
	if err := dec.Decode(data); err != nil {
		return nil, fmt.Errorf("decode response body: %w", err)
	}
	return data.Actions, nil
}

func (a *QueuedAction) exec(ctx context.Context) (QueuedActionDone, error) {
	failedResult := func(err error) QueuedActionDone {
		return QueuedActionDone{
			Id:         a.Id,
			DequeuedAt: time.Now(),
			Ret:        1,
			Stderr:     err.Error(),
		}
	}
	cmd, err := a.asCmd(ctx)
	if err != nil {
		return failedResult(err), err
	}
	log.Info().Msgf("run deueued action %d: %s", a.Id, cmd.String())
	if err := cmd.Start(); err != nil {
		return failedResult(err), err
	}
	err = cmd.Wait()

	result := QueuedActionDone{
		Id:         a.Id,
		DequeuedAt: time.Now(),
		Ret:        cmd.ExitCode(),
		Stderr:     string(cmd.Stderr()),
		Stdout:     string(cmd.Stdout()),
	}
	return result, err
}

func (a *QueuedAction) asCmd(ctx context.Context) (*command.T, error) {
	var cmdArgs []string
	if a.Svcname == "" && a.SvcId == "" {
		cmdArgs = append(cmdArgs, "node")
	} else if a.Svcname != "" {
		cmdArgs = append(cmdArgs, "svc", "-s", a.Svcname)
	}
	if a, err := args.Parse(a.Command); err != nil {
		return nil, fmt.Errorf("parse action command: %w", err)
	} else {
		cmdArgs = append(cmdArgs, a.Get()...)
	}
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(cmdArgs),
		command.WithContext(ctx),
		command.WithBufferedStderr(),
		command.WithBufferedStdout(),
	)
	return cmd, nil
}
