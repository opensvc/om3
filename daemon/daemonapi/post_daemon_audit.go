package daemonapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/rs/zerolog"
)

func (a *DaemonAPI) PostDaemonAudit(ctx echo.Context, nodename string, params api.PostDaemonAuditParams) error {
	if params.Level == nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Missing level param")
	}
	if params.Sub == nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Missing sub param")
	}
	nodename = a.parseNodename(nodename)
	if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	return a.getDaemonAudit(ctx, nodename, params)
}

func (a *DaemonAPI) getDaemonAudit(ctx echo.Context, nodename string, params api.PostDaemonAuditParams) error {
	level, err := zerolog.ParseLevel(string(*params.Level))
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "Invalid level param: %s", err)
	}

	log := LogHandler(ctx, "getDaemonAudit")
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

	subsystems := strings.Split(*params.Sub, ",")
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
			formatted, err := formatMessage(msg, messageId)
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

func formatMessage(msg plog.LogMessage, messageId uint64) ([]byte, error) {
	var b []byte
	b = append(b, []byte("id:"+strconv.FormatUint(messageId, 10))...)
	b = append(b, []byte("\ndata:")...)
	formatted, err := json.Marshal(msg)
	if err != nil {
		return []byte{}, err
	}
	b = append(b, formatted...)
	b = append(b, []byte("\n\n")...)
	return b, nil
}
