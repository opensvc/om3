package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/event/sseevent"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/streamlog"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/version"
)

// The collector speaker sends the begin and the end of the instance actions
// announced by InstanceActionPending:
//
//	begin: POST /api/instance/action, the collector answers the uuid of the
//	       action
//	end:   PUT /api/instance/action, with the uuid of the begin and the log
//	       lines read from the journal of the node the action ran on
//
// It acknowledges each with an InstanceActionSent, so the node the action
// ran on drops its pending files.

type (
	// actionToSend is the begin, or the end, of an action queued by the
	// speaker. An end replaces its begin: the collector builds the full
	// record from it.
	actionToSend struct {
		begin *msgbus.InstanceActionPending
		end   *msgbus.InstanceActionPending
	}

	// actionSentTrace is the trace of an action phase the collector
	// acknowledged, to acknowledge again an announce whose acknowledgement
	// was lost, instead of sending it twice.
	actionSentTrace struct {
		phase collector.ActionPhase
		uuid  string
		at    time.Time
	}

	// actionSendJob is an action phase to send, with the collector uuid of
	// the begin when known.
	actionSendJob struct {
		key  string
		msg  *msgbus.InstanceActionPending
		uuid string
	}

	// actionSendResult is the result of an actionSendJob. done is true when
	// the collector accepted, or refused for good, the action phase.
	actionSendResult struct {
		actionSendJob
		done bool
		err  error
	}

	// actionLogReader returns the journal records of the action a, read
	// on the node nodename.
	actionLogReader func(ctx context.Context, nodename string, a collector.Action) ([]map[string]any, error)

	// actionPost is the POST and PUT feed instance action payload.
	actionPost struct {
		Path string `json:"path"`

		// Nodename is the node the action ran on: the speaker sends the
		// actions of all the nodes, and the collector would take them for
		// the speaker ones.
		Nodename    string       `json:"nodename"`
		Action      string       `json:"action"`
		Argv        []string     `json:"argv"`
		PID         string       `json:"pid"`
		RIDs        string       `json:"rids"`
		Origin      string       `json:"origin"`
		Version     string       `json:"version"`
		Cron        bool         `json:"cron"`
		UUID        string       `json:"uuid"`
		SessionUUID string       `json:"session_uuid"`
		Begin       string       `json:"begin"`
		End         string       `json:"end"`
		Status      string       `json:"status"`
		StatusLog   string       `json:"status_log"`
		Lines       []actionLine `json:"lines"`
	}

	// actionLine is a log line of the PUT feed instance action payload.
	actionLine struct {
		Begin     string `json:"begin"`
		RID       string `json:"rid"`
		Subset    string `json:"subset"`
		PID       string `json:"pid"`
		Status    string `json:"status"`
		StatusLog string `json:"status_log"`
	}

	actionPostAccepted struct {
		UUID string `json:"uuid"`
	}

	// actionTunables are the tunables of the action sends, set from the
	// node collector configuration.
	actionTunables struct {
		// batch is the maximum number of action phases a send batch holds.
		batch int

		// postTimeout bounds a POST or a PUT feed instance action.
		postTimeout time.Duration

		// logTimeout bounds the read of the log lines of an action.
		logTimeout time.Duration
	}
)

var (
	// defaultActionBatch is the maximum number of action phases a send
	// batch holds, unless collector.action_batch says otherwise.
	defaultActionBatch = 100

	// defaultActionPostTimeout bounds a POST or a PUT feed instance action,
	// unless collector.timeout says otherwise, up to
	// maxActionPostTimeout.
	defaultActionPostTimeout = 5 * time.Second
	maxActionPostTimeout     = 20 * time.Second

	// defaultActionLogTimeout bounds the read of the log lines of an action,
	// unless collector.action_log_timeout says otherwise.
	defaultActionLogTimeout = 10 * time.Second

	// minActionTimeout is the minimum of the action timeouts.
	minActionTimeout = time.Second

	// actionLogMaxLines is the maximum number of journal records read for
	// the log lines of an action, so it also bounds the payload size.
	actionLogMaxLines = 10000

	// actionLineMaxLen is the length above which the message of a log line
	// is trimmed, as the om2 agent does.
	actionLineMaxLen = 10000

	actionLineTrimTag = " <trimmed> "

	// actionLineTimeFormat is the log line time format, the om2 agent one:
	// the collector stores it unparsed.
	actionLineTimeFormat = time.DateTime
)

// onInstanceActionPending queues the announced action phase when speaker,
// and records the announce of a local action.
func (t *T) onInstanceActionPending(c *msgbus.InstanceActionPending) {
	key := c.Action.Key()
	if c.Node == t.localhost {
		t.actionAnnouncedAt[key] = time.Now()
	}
	if !t.isSpeaker {
		return
	}
	if trace, ok := t.actionSent[key]; ok && (trace.phase == collector.ActionPhaseEnd || trace.phase == c.Phase) {
		// The collector has it: the acknowledgement was lost, or the
		// announce crossed it. Acknowledge again.
		t.log.Tracef("action %s %s already sent, acknowledge again", key, c.Phase)
		t.pubActionSent(c.Node, c.Action, trace.phase, trace.uuid)
		return
	}
	e, ok := t.actionToSend[key]
	if !ok {
		e = &actionToSend{}
		t.actionToSend[key] = e
	}
	switch c.Phase {
	case collector.ActionPhaseBegin:
		if e.end == nil {
			e.begin = c
		}
	case collector.ActionPhaseEnd:
		e.end = c
		e.begin = nil
	}
}

// onInstanceActionSent drops the pending files of the local actions the
// speaker acknowledged.
func (t *T) onInstanceActionSent(c *msgbus.InstanceActionSent) {
	if c.Node == t.localhost {
		t.onLocalInstanceActionSent(c)
	}
}

// dropActionToSend forgets the actions queued, and the ones sent, when the
// node stops being speaker. The nodes they ran on keep them pending, and
// announce them again to the next speaker.
func (t *T) dropActionToSend() {
	t.actionToSend = make(map[string]*actionToSend)
	t.actionBeginUUID = make(map[string]string)
	t.actionSent = make(map[string]actionSentTrace)
	t.actionFailed = make(map[string]struct{})
	t.actionFailLastErr = nil
	t.actionFailure.reset()
}

// pruneActionSent forgets the traces of the actions sent long enough ago
// for their node to have dropped them.
func (t *T) pruneActionSent() {
	now := time.Now()
	for key, trace := range t.actionSent {
		if now.Sub(trace.at) > actionPendingExpire {
			delete(t.actionSent, key)
			delete(t.actionBeginUUID, key)
		}
	}
}

// sendActions starts sending a batch of the queued action phases, unless a
// batch is being sent. The result comes back to the loop through
// actionSendResultC.
//
// The batch stops at the first send to retry: with the collector down, the
// others would fail too, each end after reading its log lines on its node.
func (t *T) sendActions() {
	if t.actionSending || len(t.actionToSend) == 0 || t.client == nil {
		return
	}
	jobs := t.nextActionJobs()
	if len(jobs) == 0 {
		return
	}
	t.actionSending = true
	ctx := t.ctx
	requester := t.client
	readLog := t.actionReadLog
	if readLog == nil {
		readLog = t.readActionLog
	}
	agentVersion := t.agentVersion()
	tunables := t.actionTunables
	resultC := t.actionSendResultC
	go func() {
		results := make([]actionSendResult, 0, len(jobs))
		for _, job := range jobs {
			r := sendAction(ctx, requester, readLog, agentVersion, tunables, job)
			results = append(results, r)
			if !r.done {
				break
			}
		}
		select {
		case resultC <- results:
		case <-ctx.Done():
		}
	}()
}

// nextActionJobs returns the next batch of queued action phases to send:
// the end of an action when queued, with the collector uuid of its begin
// when known, else its begin.
//
// The begins go first: they read no log lines, so they are the cheaper
// probe of a collector that may be down.
func (t *T) nextActionJobs() []actionSendJob {
	var begins, ends []actionSendJob
	for key, e := range t.actionToSend {
		switch {
		case e.end != nil:
			uuid := e.end.UUID
			if uuid == "" {
				uuid = t.actionBeginUUID[key]
			}
			ends = append(ends, actionSendJob{key: key, msg: e.end, uuid: uuid})
		case e.begin != nil:
			begins = append(begins, actionSendJob{key: key, msg: e.begin})
		}
	}
	jobs := append(begins, ends...)
	return jobs[:min(len(jobs), t.actionTunables.batch)]
}

// onActionSendResults applies the results of a send batch: an action phase
// done is dequeued and acknowledged, the others stay queued for the next
// batch.
//
// A send failing, as with a collector down, is retried: it is logged at
// debug level only, and warnActionFailed counts the action phases failing.
// An action phase done with an error, refused for good or sent without its
// log lines, is not retried: it is warned about.
func (t *T) onActionSendResults(results []actionSendResult) {
	t.actionSending = false
	now := time.Now()
	for _, r := range results {
		a := r.msg.Action
		if !r.done {
			if r.err != nil {
				t.log.Debugf("send the %s of the action %s: %s", r.msg.Phase, r.key, r.err)
				t.actionFailLastErr = r.err
			}
			t.actionFailed[r.key] = struct{}{}
			continue
		}
		if r.err != nil {
			t.log.Warnf("send the %s of the action %s: %s", r.msg.Phase, r.key, r.err)
		}
		delete(t.actionFailed, r.key)
		switch r.msg.Phase {
		case collector.ActionPhaseBegin:
			if t.isSpeaker {
				t.actionBeginUUID[r.key] = r.uuid
				t.actionSent[r.key] = actionSentTrace{phase: collector.ActionPhaseBegin, uuid: r.uuid, at: now}
				if e, ok := t.actionToSend[r.key]; ok {
					e.begin = nil
					if e.end == nil {
						delete(t.actionToSend, r.key)
					}
				}
			}
		case collector.ActionPhaseEnd:
			if t.isSpeaker {
				delete(t.actionToSend, r.key)
				delete(t.actionBeginUUID, r.key)
				t.actionSent[r.key] = actionSentTrace{phase: collector.ActionPhaseEnd, uuid: r.uuid, at: now}
			}
		}
		// Acknowledge even when no longer speaker: the collector has it.
		t.pubActionSent(r.msg.Node, a, r.msg.Phase, r.uuid)
	}
	t.warnActionFailed(now)
}

// warnActionFailed warns about the action phases whose latest send failed,
// paced by actionFailure, and tells when none fails anymore.
func (t *T) warnActionFailed(now time.Time) {
	if len(t.actionFailed) == 0 {
		t.actionFailLastErr = nil
	}
	t.actionFailure.update(t.log, now, t.actionFailLastErr, len(t.actionFailed))
}

// pubActionSent publishes the InstanceActionSent acknowledging the phase of
// the action a, which ran on nodename. The node label makes the daemon
// data forward it to the peers.
func (t *T) pubActionSent(nodename string, a collector.Action, phase collector.ActionPhase, uuid string) {
	t.publisher.Pub(&msgbus.InstanceActionSent{
		Path:   a.Path,
		Node:   nodename,
		ExecID: a.ExecID,
		Phase:  phase,
		UUID:   uuid,
	},
		pubsub.Label{"node", t.localhost},
		pubsub.Label{"namespace", a.Path.Namespace},
		pubsub.Label{"path", a.Path.String()},
	)
}

// newActionTunables returns the action send tunables of the collector
// configuration cfg: the defaults where it sets none, and the values it
// sets brought within their bounds.
func newActionTunables(cfg *collector.Config) actionTunables {
	t := actionTunables{
		batch:       defaultActionBatch,
		postTimeout: defaultActionPostTimeout,
		logTimeout:  defaultActionLogTimeout,
	}
	if cfg == nil {
		return t
	}
	if cfg.ActionBatch > 0 {
		t.batch = cfg.ActionBatch
	}
	if cfg.Timeout > 0 {
		t.postTimeout = min(max(cfg.Timeout, minActionTimeout), maxActionPostTimeout)
	}
	if cfg.ActionLogTimeout > 0 {
		t.logTimeout = max(cfg.ActionLogTimeout, minActionTimeout)
	}
	return t
}

// setActionTunables sets the action send tunables from the collector
// configuration cfg, logging the ones changed.
func (t *T) setActionTunables(cfg *collector.Config) {
	n := newActionTunables(cfg)
	if n.batch != t.actionTunables.batch {
		t.log.Infof("feeder action batch: %d", n.batch)
	}
	if n.postTimeout != t.actionTunables.postTimeout {
		t.log.Infof("feeder action post timeout: %s", n.postTimeout)
	}
	if n.logTimeout != t.actionTunables.logTimeout {
		t.log.Infof("feeder action log timeout: %s", n.logTimeout)
	}
	t.actionTunables = n
}

// agentVersion returns the agent version to send: the collector refuses an
// action whose version does not start with the agent major.
func (t *T) agentVersion() string {
	s := strings.TrimPrefix(version.Version(), "v")
	if strings.HasPrefix(s, "3.") {
		return s
	}
	return t.version
}

// sendAction sends the begin or the end of an action to the collector.
//
// A begin accepted returns the collector uuid of the action. An end carries
// the log lines read from the journal of the node the action ran on. A log
// that can not be read does not hold the end: it is sent without lines.
func sendAction(ctx context.Context, requester requester, readLog actionLogReader, agentVersion string, tunables actionTunables, job actionSendJob) actionSendResult {
	result := actionSendResult{actionSendJob: job}
	msg := job.msg
	var method string
	var lines []actionLine
	switch msg.Phase {
	case collector.ActionPhaseBegin:
		method = http.MethodPost
	case collector.ActionPhaseEnd:
		method = http.MethodPut
		logCtx, cancel := context.WithTimeout(ctx, tunables.logTimeout)
		records, err := readLog(logCtx, msg.Node, msg.Action)
		cancel()
		if err != nil {
			result.err = fmt.Errorf("read the log lines, send without: %w", err)
		}
		lines = actionLinesFromJournal(records, msg.Action.PID)
	default:
		result.err = fmt.Errorf("invalid phase %q", msg.Phase)
		return result
	}
	logErr := result.err
	result.err = nil

	body, err := json.Marshal(newActionPost(msg.Node, msg.Action, msg.Phase, job.uuid, agentVersion, lines))
	if err != nil {
		result.err = fmt.Errorf("encode request body: %w", err)
		return result
	}
	path := oc3path.FeedInstanceAction
	postCtx, cancel := context.WithTimeout(ctx, tunables.postTimeout)
	defer cancel()
	req, err := requester.NewRequestWithContext(postCtx, method, path, bytes.NewReader(body))
	if err != nil {
		result.err = fmt.Errorf("%s %s create request: %w", method, path, err)
		return result
	}
	resp, err := requester.Do(req)
	if err != nil {
		result.err = errors.Join(logErr, fmt.Errorf("%s %s: %w", method, path, err))
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusAccepted:
		result.done = true
		if msg.Phase == collector.ActionPhaseBegin {
			var accepted actionPostAccepted
			if err := json.NewDecoder(resp.Body).Decode(&accepted); err == nil {
				result.uuid = accepted.UUID
			}
		}
	case http.StatusBadRequest, http.StatusForbidden:
		// refused for good, as the action of a node the collector does not
		// know in the cluster: sending it again would not do better
		result.done = true
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		result.err = fmt.Errorf("%s %s refused, dropped: %s", method, path, bytes.TrimSpace(b))
	default:
		result.err = fmt.Errorf("%s %s unexpected status code: wanted %d got %d", method, path, http.StatusAccepted, resp.StatusCode)
	}
	if result.err == nil {
		result.err = logErr
	}
	return result
}

// newActionPost returns the feed instance action payload of the phase of
// the action a, which ran on nodename.
//
// The end carries the collector uuid of its begin, so the collector merges
// both when the begin is not processed yet. When the begin uuid is unknown,
// the exec id stands for it: it is unique per action, so two ends of an
// object never share a collector queue entry.
func newActionPost(nodename string, a collector.Action, phase collector.ActionPhase, uuid, agentVersion string, lines []actionLine) actionPost {
	if lines == nil {
		lines = []actionLine{}
	}
	argv := a.Argv
	if argv == nil {
		argv = []string{}
	}
	post := actionPost{
		Path:        a.Path.String(),
		Nodename:    nodename,
		Action:      a.Action,
		Argv:        argv,
		PID:         strconv.Itoa(a.PID),
		RIDs:        a.RIDs,
		Origin:      a.Origin,
		Version:     agentVersion,
		Cron:        false,
		SessionUUID: a.SessionID.String(),
		Begin:       a.Begin.Format(time.RFC3339Nano),
		Lines:       lines,
	}
	if phase == collector.ActionPhaseEnd {
		post.End = a.End.Format(time.RFC3339Nano)
		post.Status = a.Status
		post.UUID = uuid
		if post.UUID == "" {
			post.UUID = a.ExecID.String()
		}
	}
	return post
}

// actionLinesFromJournal converts the journal records of an action to the
// log lines of its end, as the om2 agent does:
//
//   - the level gives the status: error and above is err, warn is warn,
//     others are ok
//   - a message above actionLineMaxLen is trimmed in its middle
//   - consecutive records of the same resource and status are merged, their
//     messages joined by a new line
func actionLinesFromJournal(records []map[string]any, pid int) []actionLine {
	lines := make([]actionLine, 0)
	pidS := strconv.Itoa(pid)
	for _, record := range records {
		at, level, rid, message, ok := parseJournalRecord(record)
		if !ok {
			continue
		}
		status := "ok"
		switch level {
		case "error", "fatal", "panic":
			status = "err"
		case "warn":
			status = "warn"
		case "debug", "trace":
			continue
		}
		message = trimActionLineMessage(message)
		if n := len(lines); n > 0 && lines[n-1].RID == rid && lines[n-1].Status == status {
			lines[n-1].StatusLog += "\n" + message
			continue
		}
		var begin string
		if !at.IsZero() {
			begin = at.Local().Format(actionLineTimeFormat)
		}
		lines = append(lines, actionLine{
			Begin:     begin,
			RID:       rid,
			PID:       pidS,
			Status:    status,
			StatusLog: message,
		})
	}
	return lines
}

// parseJournalRecord returns the time, level, resource id and message of a
// journal record, read from the zerolog record the JSON field holds, or
// from the journal fields when it holds none.
func parseJournalRecord(record map[string]any) (at time.Time, level, rid, message string, ok bool) {
	if s, isString := record["JSON"].(string); isString {
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			if v, isString := m["time"].(string); isString {
				at, _ = time.Parse(time.RFC3339Nano, v)
			}
			level, _ = m["level"].(string)
			rid, _ = m["rid"].(string)
			message, _ = m["message"].(string)
			return at, level, rid, message, true
		}
	}
	message, ok = record["MESSAGE"].(string)
	if !ok {
		return
	}
	rid, _ = record["RID"].(string)
	switch record["PRIORITY"] {
	case "0", "1", "2", "3":
		level = "error"
	case "4":
		level = "warn"
	case "7":
		level = "debug"
	default:
		level = "info"
	}
	if s, isString := record["__REALTIME_TIMESTAMP"].(string); isString {
		if usec, err := strconv.ParseInt(s, 10, 64); err == nil {
			at = time.UnixMicro(usec)
		}
	}
	return at, level, rid, message, true
}

func trimActionLineMessage(s string) string {
	if len(s) <= actionLineMaxLen {
		return s
	}
	head := actionLineMaxLen / 2
	tail := head - len(actionLineTrimTag)
	return s[:head] + actionLineTrimTag + s[len(s)-tail:]
}

// readActionLog returns the journal records of the action a, read on the
// node nodename: from the local journal, or from the node logs api of the
// peer.
func (t *T) readActionLog(ctx context.Context, nodename string, a collector.Action) ([]map[string]any, error) {
	filters := []string{
		"EXEC_ID=" + a.ExecID.String(),
		"OBJ_PATH=" + a.Path.String(),
	}
	if nodename == t.localhost {
		return readLocalActionLog(ctx, filters)
	}
	return readPeerActionLog(ctx, nodename, filters)
}

func readLocalActionLog(ctx context.Context, filters []string) ([]map[string]any, error) {
	stream := streamlog.NewStream()
	if err := stream.Start(streamlog.StreamConfig{
		Lines:   actionLogMaxLines,
		Matches: filters,
	}); err != nil {
		return nil, err
	}
	records := make([]map[string]any, 0)
	var errs error
	for {
		select {
		case <-ctx.Done():
			// The stream senders block until read: drain them until the
			// killed journalctl is waited.
			_ = stream.Stop()
			go func() {
				for {
					select {
					case <-stream.Events():
					case err := <-stream.Errors():
						if err == nil {
							return
						}
					}
				}
			}()
			return records, errors.Join(errs, ctx.Err())
		case ev := <-stream.Events():
			records = append(records, ev.M)
		case err := <-stream.Errors():
			if err == nil {
				return records, errs
			}
			errs = errors.Join(errs, err)
		}
	}
}

func readPeerActionLog(ctx context.Context, nodename string, filters []string) ([]map[string]any, error) {
	tk, err := daemonauth.CreateNodeToken()
	if err != nil {
		return nil, err
	}
	c, err := client.New(
		client.WithURL(daemonsubsystem.PeerURL(nodename)),
		client.WithBearer(tk),
		client.WithCertificate(daemonenv.CertChainFile()),
		client.WithTimeout(0),
	)
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}
	lines := actionLogMaxLines
	resp, err := c.GetNodeLogs(ctx, nodename, &api.GetNodeLogsParams{
		Filter: &filters,
		Lines:  &lines,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	reader := sseevent.NewReadCloser(resp.Body)
	reader.SetContext(ctx)
	defer func() { _ = reader.Close() }()
	records := make([]map[string]any, 0)
	for {
		ev, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return records, nil
		} else if err != nil {
			return records, err
		} else if ev == nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.Data, &m); err != nil {
			continue
		}
		records = append(records, m)
	}
}
