package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/xmap"
)

type (
	// postFeedDaemonPing describes the POST feed daemon ping payload
	postFeedDaemonPing struct {
		Nodes   []string `json:"nodes"`
		Objects []string `json:"Objects"`
		Version string   `json:"version"`
	}

	// postFeedDaemonStatus describes the POST feed daemon status payload
	postFeedDaemonStatus struct {
		PreviousUpdatedAt time.Time `json:"previous_updated_at"`
		UpdatedAt         time.Time `json:"updated_at"`
		// The cluster dataset is not a field here: see reader. It is
		// spliced into the body rather than handed to encoding/json.
		Changes []string `json:"changes"`
		Version string   `json:"version"`
	}

	// PingStatusResponse used to decode the response of post feed daemon status and ping
	PingStatusResponse struct {
		ObjectWithoutConfig  *[]string `json:"object_without_config"`
		NodeWithActionQueued *[]string `json:"node_with_action_queued"`
	}

	// daemonDataJob is a daemon status or ping the loop hands to a sender.
	daemonDataJob struct {
		// epoch is the feed epoch the job was cut in.
		epoch uint64

		// now is the time the job was cut: the UpdatedAt of the status,
		// and the previousUpdatedAt of the next one once accepted.
		now time.Time

		// status is the status to send, nil for a ping.
		status *postFeedDaemonStatus

		// ping is the ping to send, nil for a status.
		ping *postFeedDaemonPing

		// changes are the daemonStatusChange keys the status reports,
		// swapped out of the loop for the send.
		changes map[string]struct{}
	}

	// daemonDataResult is the outcome of a daemonDataJob send.
	daemonDataResult struct {
		daemonDataJob

		// respondedAt is the time of the collector response, zero
		// without response.
		respondedAt time.Time

		statusCode int
		response   PingStatusResponse
		decodeErr  error

		// err is a send failing: no response, or not an accepted or a
		// resync one.
		err error
	}
)

// reader returns the request body, with data spliced in as its "data"
// field, without encoding/json ever seeing the dataset.
//
// A json.RawMessage field is the obvious way and is the wrong one:
// Marshal compacts a RawMessage, validating and copying it, and on a
// 600KB dataset that costs more than the deep copy it was meant to
// spare. Measured over a 103 object cluster: 8.2ms for the RawMessage
// field, 5.5ms for marshalling the dataset struct outright, 4.3ms for
// splicing.
//
// The envelope is a json object with at least one field, none of them
// omitempty, so it ends in "}" and the dataset can go in before it.
func (b postFeedDaemonStatus) reader(data []byte) (io.Reader, error) {
	envelope, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	if n := len(envelope); n < 2 || envelope[n-1] != '}' {
		return nil, fmt.Errorf("unexpected daemon status envelope: %.32s", envelope)
	}
	return io.MultiReader(
		bytes.NewReader(envelope[:len(envelope)-1]),
		strings.NewReader(`,"data":`),
		bytes.NewReader(data),
		strings.NewReader("}"),
	), nil
}

// sendDaemonData starts the send of the daemon status when changes are
// pending, else of a daemon ping, unless one is in flight or throttled. The
// result comes back to the loop through sendResultC.
//
// The status reports the pending changes, swapped out of daemonStatusChange
// for the send: the changes coming meanwhile are the next status ones, and a
// send failing gives them back.
func (t *T) sendDaemonData() {
	if t.daemonDataSending {
		return
	}
	if t.client == nil {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
		return
	}
	now := time.Now()
	job := daemonDataJob{epoch: t.feedEpoch, now: now}
	if len(t.daemonStatusChange) > 0 {
		if !t.feedStatusAt.IsZero() && now.Sub(t.feedStatusAt) < t.statusDelay {
			t.log.Tracef("post daemon status throttled")
			return
		}
		job.changes = t.daemonStatusChange
		t.daemonStatusChange = make(map[string]struct{})
		job.status = &postFeedDaemonStatus{
			PreviousUpdatedAt: t.previousUpdatedAt,
			UpdatedAt:         now,
			Changes:           xmap.Keys(job.changes),
			Version:           t.version,
		}
	} else {
		if now.Sub(t.feedPingOrStatusAt) < t.pingInterval {
			t.log.Tracef("post daemon ping throttled")
			return
		}
		job.ping = t.postPingBody()
	}
	t.daemonDataSending = true
	t.daemonStatusInFlight = len(job.changes)
	ctx, requester, clusterData, timeout := t.ctx, t.client, t.clusterData, t.postTimeout
	t.goSend(func() sendResult {
		return sendDaemonData(ctx, requester, clusterData, timeout, job)
	})
}

// sendDaemonData sends the daemon status or ping of job to the collector.
// The status carries the cluster dataset read now, after the changes it
// reports were swapped out: a change coming in between is in both this one
// and the next one, never in none.
func sendDaemonData(ctx context.Context, requester requester, clusterData clusterDataer, timeout time.Duration, job daemonDataJob) (r daemonDataResult) {
	r.daemonDataJob = job
	method, path := http.MethodPost, job.path()
	var body io.Reader
	if job.status != nil {
		data, err := clusterData.ClusterDataJSON()
		if err != nil {
			r.err = fmt.Errorf("%s %s marshal cluster data: %w", method, path, err)
			return
		}
		if len(data) == 0 {
			r.err = fmt.Errorf("%s %s abort on empty cluster data", method, path)
			return
		}
		if body, err = job.status.reader(data); err != nil {
			r.err = fmt.Errorf("%s %s body: %w", method, path, err)
			return
		}
	} else {
		b, err := json.Marshal(job.ping)
		if err != nil {
			r.err = fmt.Errorf("%s %s body: %w", method, path, err)
			return
		}
		body = bytes.NewReader(b)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := requester.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		r.err = fmt.Errorf("%s %s create request: %w", method, path, err)
		return
	}
	if job.status != nil {
		req.Header.Set(headerPreviousUpdatedAt, job.status.PreviousUpdatedAt.Format(time.RFC3339Nano))
	}
	resp, err := requester.Do(req)
	if err != nil {
		r.err = fmt.Errorf("%s %s: %w", method, path, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	r.respondedAt = time.Now()
	r.statusCode = resp.StatusCode
	switch resp.StatusCode {
	case http.StatusAccepted:
		r.response, r.decodeErr = decodePingStatusResponse(resp.Body)
	case job.resyncCode():
	default:
		r.err = fmt.Errorf("%s %s unexpected status code %d", method, path, resp.StatusCode)
	}
	return
}

// apply applies the result of a daemon status or ping send. A result of a
// send cut before a resync or a speaker change is dropped.
func (r daemonDataResult) apply(t *T) {
	t.daemonDataSending = false
	t.daemonStatusInFlight = 0
	if r.epoch != t.feedEpoch {
		t.log.Debugf("drop the result of a daemon %s sent before a resync or a speaker change", r.kind())
		return
	}
	if !r.respondedAt.IsZero() {
		t.feedPingOrStatusAt = r.respondedAt
		if r.status != nil {
			t.feedStatusAt = r.respondedAt
		}
	}
	now := time.Now()
	switch {
	case r.err != nil:
		// Sent again with the changes coming meanwhile.
		for k := range r.changes {
			t.daemonStatusChange[k] = struct{}{}
		}
		t.log.Debugf("send daemon %s: %s", r.kind(), r.err)
		t.daemonStatusFailure.update(t.log, now, r.err, 0)
	case r.statusCode == r.resyncCode():
		// The collector detects a break of the changes chain: send the
		// full dataset.
		t.log.Infof("%s %s status code %d", http.MethodPost, r.path(), r.statusCode)
		t.initChanges()
		t.daemonStatusFailure.update(t.log, now, nil, 0)
	default:
		t.previousUpdatedAt = r.now
		t.daemonStatusFailure.update(t.log, now, nil, 0)
		t.handlePingOrStatusResponse(r)
	}
}

func (j daemonDataJob) kind() string {
	if j.status != nil {
		return "status"
	}
	return "ping"
}

func (j daemonDataJob) path() string {
	if j.status != nil {
		return oc3path.FeedDaemonStatus
	}
	return oc3path.FeedDaemonPing
}

// resyncCode is the status code of the collector detecting a break of the
// changes chain, asking for the full dataset.
func (j daemonDataJob) resyncCode() int {
	if j.status != nil {
		return http.StatusConflict
	}
	return http.StatusNoContent
}

func (t *T) postPingBody() *postFeedDaemonPing {
	objects := make([]string, 0, len(t.clusterObject))
	for k := range t.clusterObject {
		objects = append(objects, k)
	}
	nodes := make([]string, 0, len(t.clusterNode))
	for k := range t.clusterNode {
		nodes = append(nodes, k)
	}
	return &postFeedDaemonPing{
		Nodes:   nodes,
		Objects: objects,
		Version: t.version,
	}
}

func decodePingStatusResponse(r io.Reader) (obj PingStatusResponse, err error) {
	err = json.NewDecoder(r).Decode(&obj)
	return
}

// handlePingOrStatusResponse queues the instance configs the collector says
// it misses, and asks the nodes with actions queued to dequeue them.
func (t *T) handlePingOrStatusResponse(r daemonDataResult) {
	method, path := http.MethodPost, r.path()
	if r.decodeErr != nil {
		t.log.Warnf("%s %s status code %d can't decode response: %s", method, path, r.statusCode, r.decodeErr)
		return
	}
	data := r.response
	if data.ObjectWithoutConfig != nil {
		if addedPath, err := t.updateConfigToSend(*data.ObjectWithoutConfig); err != nil {
			t.log.Warnf("%s %s status code %d can't detect missing instance config: %s", method, path, r.statusCode, err)
		} else if len(addedPath) > 0 {
			t.log.Infof("%s %s status code %d got missing instance config: %s", method, path, r.statusCode, addedPath)
		} else {
			t.log.Debugf("%s %s status code %d", method, path, r.statusCode)
		}
	}
	if data.NodeWithActionQueued != nil {
		for _, nodename := range *data.NodeWithActionQueued {
			if _, ok := t.clusterNode[nodename]; !ok {
				t.log.Warnf("%s %s delegate dequeue action for %s: not a cluster node", method, path, nodename)
				continue
			}
			t.log.Infof("%s %s delegate dequeue action for %s", method, path, nodename)
			t.startDequeueAction(nodename)
		}
	}
}

// updateConfigToSend updates t.objectConfigToSend with missing
// paths that are found from the decoded r into the ObjectWithoutConfig.
// It returns the added paths.
//
// Invalid paths are ignored.
//
// Too recently sent paths are delayed to the next iteration with delay value:
// objectConfigToSendMinDelay for the following reasons:
//   - oc3 api ObjectWithoutConfig values have been computed during last
//     job calls => it may contain already POSTED object paths
//   - collector-speaker may be called after POST ping or status
//
// Example:
//   - @collector-speaker: POST oc3 ping or status
//   - @oc3-api async job ping or status
//     respond with previous job ping or status ObjectWithoutConfig "obj1"
//   - @collector-speaker add "obj1" to list of configs to POST on next ticker
//   - @oc3-worker job ping or status prepare next ObjectWithoutConfig "obj1"
//   - @collector-speaker on ticker POST configs for path "obj1"
//     and update t.objectConfigSent["obj1"].SentAt = now()
//   - @collector-speaker: POST oc3 ping or status
//   - @oc3-api async job ping or status
//   - @oc3-api prepare job ping
//     respond with previous job ping or status ObjectWithoutConfig "obj1"
//   - @collector-speaker don't add "obj1" because of the objectConfigToSendMinDelay
func (t *T) updateConfigToSend(objWithoutConfig []string) (added []naming.Path, err error) {
	for _, s := range objWithoutConfig {
		p, err := naming.ParsePath(s)
		if err != nil {
			continue
		}
		if _, ok := t.objectConfigToSend[p]; !ok {
			toAdd := true
			if sent, ok := t.objectConfigSent[p]; ok {
				if time.Now().Before(sent.SentAt.Add(t.objectConfigToSendMinDelay)) {
					toAdd = false
					t.log.Tracef("throttled instance config send %s", p)
				}
			}
			if toAdd {
				if ok = t.dropInstanceConfigSentFlag(objectConfigSent{path: p}); !ok {
					t.log.Tracef("mark need instance config send %s", p)
				}
				delete(t.objectConfigSent, p)
				t.objectConfigToSend[p] = nil
				added = append(added, p)
			}
		}
	}
	return
}

// dropInstanceConfigSentFlag drops the instanceConfigSentFlag. It returns true
// if existing instance config sent path has been removed, or false when flag is
// absent.
func (t *T) dropInstanceConfigSentFlag(sent objectConfigSent) bool {
	if err := sent.drop(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false
		} else {
			t.log.Errorf("drop instance config sent flag %s: %s", sent.path, err)
			return false
		}
	} else {
		t.log.Tracef("dropped previous instance config sent flag %s", sent.path)
		return true
	}
}

// startDequeueAction asks nodename to dequeue its queued actions, without
// waiting: a failure is logged only.
func (t *T) startDequeueAction(nodename string) {
	ctx, log := t.ctx, t.log
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		if err := dequeueAction(ctx, nodename); err != nil {
			log.Warnf("delegate dequeue action for %s: %s", nodename, err)
		} else {
			log.Infof("dequeue action accepted by %s", nodename)
		}
	}()
}

func dequeueAction(ctx context.Context, nodename string) error {
	options := []funcopt.O{
		client.WithURL(daemonsubsystem.PeerURL(nodename)),
		client.WithServerName(nodename),
		client.WithBearerFunc(daemonauth.CreateNodeToken),
		client.WithTimeout(peerRequestTimeout),
	}
	cli, err := client.New(options...)
	if err != nil {
		return fmt.Errorf("new client: %s", err)
	}
	ctx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()
	resp, err := cli.PostPeerActionDequeue(ctx, nodename, nil)
	if err != nil {
		return fmt.Errorf("post action dequeue %s: %w", nodename, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post action dequeue %s: status code %d", nodename, resp.StatusCode)
	}
	return nil
}
