package daemonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (a *DaemonAPI) PostDaemonAudit(ctx echo.Context, nodename string, params api.PostDaemonAuditParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if params.Level == nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Missing level param")
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost || nodename == "localhost" {
		return a.getLocalDaemonAudit(ctx, nodename, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	return a.getPeerDaemonAudit(ctx, nodename, params)
}

func (a *DaemonAPI) getPeerDaemonAudit(ctx echo.Context, nodename string, params api.PostDaemonAuditParams) error {
	request := ctx.Request()
	evCtx := request.Context()
	log := LogHandler(ctx, "getPeerDaemonAudit")

	c, err := a.newProxyClient(ctx, nodename, client.WithTimeout(0))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	w := ctx.Response()
	resp, err := c.PostDaemonAudit(evCtx, nodename, &params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			return err
		}
	}

	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	w.WriteHeader(http.StatusOK)
	w.Flush()

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("response from %s body close: %s", nodename, err)
		}
	}()
	return streamCopyFlush(evCtx, w, resp.Body)
}

func (a *DaemonAPI) getLocalDaemonAudit(ctx echo.Context, nodename string, params api.PostDaemonAuditParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
		return err
	} else if !v {
		return nil
	}

	var messageId uint64

	user := userFromContext(ctx)

	level, err := zerolog.ParseLevel(string(*params.Level))
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid level param: %s", err)
	}

	log := LogHandler(ctx, "getLocalDaemonAudit")
	var preempt bool
	if params.Preempt == nil || !*params.Preempt {
		preempt = false
	} else {
		preempt = true
	}

	q := make(chan plog.LogMessage, 1000)
	preemptC := make(chan struct{})

	var subsystems []string
	if params.Sub != nil {
		subsystems = *params.Sub
	}

	labels := []pubsub.Label{
		{"node", nodename},
		labelOriginAPI,
	}

	// activate tells the subsystems to send their logs to q. The registry
	// calls it before another session can begin, so a session preempted
	// now can not tell them after the session preempting it.
	var busAudit bool
	activate := func() {
		if len(subsystems) == 0 || slices.Contains(subsystems, "pubsub") {
			busAudit = a.Bus.AuditStart(q) == nil
		}
		a.Bus.Pub(&msgbus.AuditStart{Q: q, Subsystems: subsystems}, labels...)
	}
	if a.AuditRegistry != nil {
		if sess, ok := a.AuditRegistry.Begin(q, subsystems, preemptC, user.Username, preempt, activate); !ok {
			return JSONProblemf(ctx, http.StatusConflict, "Audit already active", "refused, audit session is already running for user %s", sess.User)
		}
		defer a.AuditRegistry.Stop(q)
	} else {
		activate()
	}
	if busAudit {
		defer a.Bus.AuditStop(q)
	}

	request := ctx.Request()
	evCtx := request.Context()

	w := ctx.Response()
	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	w.WriteHeader(http.StatusOK)
	w.Flush()

	log.Infof("publish audit start session %s", uuidFromContext(ctx))
	defer log.Infof("publish audit stop session %s", uuidFromContext(ctx))

	write := func(event string, msg plog.LogMessage) error {
		formatted, err := formatMessage(event, msg, messageId, nodename)
		if err != nil {
			return fmt.Errorf("failed to format log message: %v", err)
		}
		_, err = w.Write(formatted)
		messageId++
		if err != nil {
			return fmt.Errorf("failed to write message: %v", err)
		}
		w.Flush()
		return nil
	}

	defer a.Bus.Pub(&msgbus.AuditStop{Q: q, Subsystems: subsystems}, labels...)
	msg := plog.LogMessage{
		Level:     zerolog.InfoLevel,
		Timestamp: time.Now(),
		Message:   "daemon audit: audit started",
	}
	if err = write("", msg); err != nil {
		log.Warnf("%s", err)
		return nil
	}
	for {
		select {
		case msg := <-q:
			if msg.Level < level {
				continue
			}
			if err = write("", msg); err != nil {
				log.Warnf("%s", err)
				return nil
			}
		case <-evCtx.Done():
			return nil
		case <-preemptC:
			msg := plog.LogMessage{
				Level:     zerolog.WarnLevel,
				Timestamp: time.Now(),
				Message:   "daemon audit: session preempted by another session",
			}
			// The event tells the client not to reconnect: it would
			// preempt the session that preempted it.
			if err := write(api.AuditEventPreempted, msg); err != nil {
				log.Warnf("%s", err)
			}
			return nil
		}
	}
}

// formatMessage formats a log message as a server-sent event, named event
// when it is not empty.
func formatMessage(event string, msg plog.LogMessage, messageId uint64, nodename string) ([]byte, error) {
	var b []byte
	b = append(b, []byte("id:"+strconv.FormatUint(messageId, 10))...)
	if event != "" {
		b = append(b, []byte("\nevent:"+event)...)
	}
	b = append(b, []byte("\ndata:")...)
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	msg.Message = fmt.Sprintf("%s: %s", nodename, msg.Message)
	err := encoder.Encode(msg)
	if err != nil {
		return []byte{}, err
	}
	b = append(b, buf.Bytes()...)
	b = append(b, []byte("\n\n")...)
	return b, nil
}

func streamCopyFlush(ctx context.Context, w *echo.Response, src io.Reader) error {
	buf := make([]byte, 32*1024)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			w.Flush()
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}
