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
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/rs/zerolog"
)

func (a *DaemonAPI) PostDaemonAudit(ctx echo.Context, nodename string, params api.PostDaemonAuditParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if params.Level == nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Missing level param")
	}
	if params.Sub == nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Missing sub param")
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

	level, err := zerolog.ParseLevel(string(*params.Level))
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid level param: %s", err)
	}

	log := LogHandler(ctx, "getLocalDaemonAudit")
	request := ctx.Request()
	evCtx := request.Context()

	w := ctx.Response()
	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	w.WriteHeader(http.StatusOK)
	w.Flush()

	q := make(chan plog.LogMessage, 1000)
	labels := []pubsub.Label{
		{"node", nodename},
		labelOriginAPI,
	}

	var subsystems []string
	if *params.Sub != "" {
		subsystems = strings.Split(*params.Sub, ",")
	}

	if len(subsystems) == 0 || slices.Contains(subsystems, "pubsub") {
		a.Auditor.AuditStart(q)
		defer a.Auditor.AuditStop(q)
	}
	a.Publisher.Pub(&msgbus.AuditStart{Q: q, Subsystems: subsystems}, labels...)
	log.Infof("Publish audit start")

	defer a.Publisher.Pub(&msgbus.AuditStop{Q: q, Subsystems: subsystems}, labels...)
	var messageId uint64
	for {
		select {
		case msg := <-q:
			if msg.Level < level {
				continue
			}
			formatted, err := formatMessage(msg, messageId, nodename)
			if err != nil {
				log.Warnf("Failed to format log message: %v", err)
				return nil
			}
			_, err = w.Write(formatted)
			messageId++
			if err != nil {
				log.Warnf("Failed to write message: %v", err)
				return nil
			}
			w.Flush()
		case <-evCtx.Done():
			return nil
		}
	}
}

func formatMessage(msg plog.LogMessage, messageId uint64, nodename string) ([]byte, error) {
	var b []byte
	b = append(b, []byte("id:"+strconv.FormatUint(messageId, 10))...)
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
