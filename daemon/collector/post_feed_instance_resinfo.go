package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	// resInfoPost is the POST feed instance resinfo payload.
	resInfoPost struct {
		Info []resource.Info `json:"info"`
		Path string          `json:"path"`

		// Nodename is the node of the instance: the speaker sends the
		// resource info of all the nodes, and the collector would take them
		// for the speaker ones.
		Nodename string  `json:"nodename"`
		Topology *string `json:"topology,omitempty"`
	}

	// resInfoSent describes the resource info sent to the collector db. It
	// prevents re-sending unchanged info and is dumped to
	// <var>/collector/resinfo_sent/<fqdn>@<node>.json so the cache survives a
	// daemon restart.
	resInfoSent struct {
		SentAt   time.Time `json:"sent_at"`
		Checksum string    `json:"csum"`

		path     naming.Path
		nodename string

		// cacheFile is the file path to store the resInfoSent struct
		cacheFile string
	}
)

// resInfoKinds are the object kinds that have resources, and so resource
// info to report. The others are datastores and configurations: asking
// the local instance for info it has no way to hold answers "object does
// not support resource info", and asking a peer's api for it answers 400.
var resInfoKinds = naming.NewKinds(naming.KindSvc, naming.KindVol)

// seedResInfoToSend queues the instances we have no sent trace for, so a node
// that becomes speaker still reports resource info refreshed while it was not
// speaking.
//
// Instances with a sent trace are skipped: they wait for the next
// InstanceResourceInfoUpdated signal or the next scheduled refresh. This bounds
// the fetch burst on speaker change.
func (t *T) seedResInfoToSend() {
	for _, v := range instance.StatusData.GetAll() {
		if !resInfoKinds.Has(v.Path.Kind) {
			continue
		}
		i := instance.InstanceString(v.Path, v.Node)
		if _, ok := t.resInfoToSend[i]; ok {
			continue
		}
		if _, ok := t.resInfoSent[i]; ok {
			continue
		}
		sent := resInfoSent{path: v.Path, nodename: v.Node}
		if err := sent.read(); err == nil {
			t.resInfoSent[i] = sent
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.log.Warnf("can't read sent resource info flag for %s: %s", i, err)
		}
		t.log.Tracef("seed resource info to send for %s", i)
		t.resInfoToSend[i] = &msgbus.InstanceResourceInfoUpdated{Path: v.Path, Node: v.Node}
	}
}

// sendResInfoChange starts the send of a batch of the queued resource infos,
// unless one is in flight. The result comes back to the loop through
// sendResultC.
//
// A resource info sent, or refused for good, is dequeued. A send failing
// stops the batch: it and the ones not tried stay queued for a later tick,
// as with the collector down the others would fail too, each after fetching
// its info on its node.
func (t *T) sendResInfoChange() {
	if t.resInfoSending || len(t.resInfoToSend) == 0 || t.client == nil {
		return
	}
	jobs := make([]resInfoJob, 0, min(len(t.resInfoToSend), sendBatchMax))
	for i, v := range t.resInfoToSend {
		if len(jobs) == sendBatchMax {
			break
		}
		jobs = append(jobs, resInfoJob{key: i, queued: v})
	}
	t.resInfoSending = true
	epoch, ctx, requester, timeout, localhost := t.feedEpoch, t.ctx, t.client, t.postTimeout, t.localhost
	get := t.resInfoGet
	if get == nil {
		get = func(p naming.Path, nodename string) (resource.Infos, error) {
			return getResInfo(ctx, localhost, p, nodename)
		}
	}
	t.goSend(func() sendResult {
		r := resInfoResult{epoch: epoch}
		for _, job := range jobs {
			item := resInfoItem{resInfoJob: job}
			v := job.queued
			if infos, err := get(v.Path, v.Node); err != nil {
				item.outcome, item.err = sendDropped, err
			} else {
				item.outcome, item.err = doPostResInfo(ctx, requester, timeout, v, infos)
				if item.outcome == sendDone {
					item.sentAt = time.Now()
				}
			}
			r.items = append(r.items, item)
			if item.outcome == sendFailed {
				break
			}
		}
		return r
	})
}

// apply applies the result of a batch of resource infos sent. An entry
// replaced in flight stays queued for its newer version, and one deleted in
// flight, with its instance, is not recorded sent.
func (r resInfoResult) apply(t *T) {
	t.resInfoSending = false
	if r.epoch != t.feedEpoch {
		t.log.Debugf("drop the result of resource infos sent before a resync or a speaker change")
		return
	}
	var failErr error
	for _, item := range r.items {
		i, v := item.key, item.queued
		switch item.outcome {
		case sendFailed:
			// Kept queued, retried on a later tick.
			t.log.Debugf("post resource info %s: %s", i, item.err)
			failErr = item.err
			continue
		case sendDropped:
			// skip os.ErrNotExist: the instance may never have refreshed
			// its cache, or may have been deleted.
			if errors.Is(item.err, errCollectorRefused) {
				// refused for good, as the info of a node the collector
				// does not know in the cluster
				t.log.Warnf("post resource info %s: %s", i, item.err)
			} else if !errors.Is(item.err, os.ErrNotExist) {
				t.log.Warnf("skip send resource info %s: %s", i, item.err)
			}
		}
		if queued, ok := t.resInfoToSend[i]; !ok || queued != v {
			continue
		}
		delete(t.resInfoToSend, i)
		if item.outcome == sendDone {
			sent := resInfoSent{path: v.Path, nodename: v.Node, Checksum: v.Checksum, SentAt: item.sentAt}
			if err := sent.write(); err != nil {
				// the collector has it: not a send to retry
				t.log.Warnf("save the sent resource info flag for %s: %s", i, err)
			}
			t.resInfoSent[i] = sent
		}
	}
	t.resInfoFailure.update(t.log, time.Now(), failErr, len(t.resInfoToSend))
}

// getResInfo returns the resource info key-values of the p instance on nodename,
// read from the local cache when nodename is localhost, else fetched from the
// nodename daemon api.
func getResInfo(ctx context.Context, localhost string, p naming.Path, nodename string) (resource.Infos, error) {
	if nodename == localhost {
		return getLocalResInfo(p)
	}
	return getPeerResInfo(ctx, p, nodename)
}

func getLocalResInfo(p naming.Path) (resource.Infos, error) {
	type loadResInfoer interface {
		LoadResInfo() (resource.Infos, error)
	}
	infos := resource.NewInfos(p)
	o, err := object.New(p)
	if err != nil {
		return infos, err
	}
	i, ok := o.(loadResInfoer)
	if !ok {
		return infos, fmt.Errorf("object does not support resource info")
	}
	return i.LoadResInfo()
}

func getPeerResInfo(ctx context.Context, p naming.Path, nodename string) (resource.Infos, error) {
	infos := resource.NewInfos(p)
	c, err := client.New(
		client.WithURL(daemonsubsystem.PeerURL(nodename)),
		client.WithServerName(nodename),
		client.WithBearerFunc(daemonauth.CreateNodeToken),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
	if err != nil {
		return infos, fmt.Errorf("new client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()
	resp, err := c.GetInstanceResourceInfoWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name)
	if err != nil {
		return infos, err
	}
	switch {
	case resp.JSON200 != nil:
		return resInfoFromAPI(p, resp.JSON200.Items), nil
	case resp.StatusCode() == http.StatusNotFound:
		return infos, os.ErrNotExist
	default:
		return infos, fmt.Errorf("unexpected response: %s", resp.Status())
	}
}

// resInfoFromAPI rebuilds the resource.Infos from the flat key-value items the
// api serves, preserving the item order so the collector sees a stable rid
// ordering.
func resInfoFromAPI(p naming.Path, items api.ResourceInfoItems) resource.Infos {
	infos := resource.NewInfos(p)
	index := make(map[string]int)
	for _, item := range items {
		i, ok := index[item.Rid]
		if !ok {
			infos.Resources = append(infos.Resources, resource.Info{RID: item.Rid})
			i = len(infos.Resources) - 1
			index[item.Rid] = i
		}
		infos.Resources[i].Keys = append(infos.Resources[i].Keys, resource.InfoKey{
			Key:   item.Key,
			Value: item.Value,
		})
	}
	return infos
}

// doPostResInfo sends the resource info infos of the instance of v to the
// collector, and returns the outcome.
func doPostResInfo(ctx context.Context, requester requester, timeout time.Duration, v *msgbus.InstanceResourceInfoUpdated, infos resource.Infos) (sendOutcome, error) {
	var (
		method = http.MethodPost
		path   = oc3path.FeedInstanceResinfo
	)
	data := resInfoPost{
		Info:     infos.Resources,
		Path:     v.Path.String(),
		Nodename: v.Node,
	}
	if cfg := instance.ConfigData.GetByPathAndNode(v.Path, v.Node); cfg != nil {
		topology := cfg.Topology.String()
		data.Topology = &topology
	}
	b, err := json.Marshal(data)
	if err != nil {
		return sendDropped, fmt.Errorf("encode request body: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := requester.NewRequestWithContext(ctx, method, path, bytes.NewBuffer(b))
	if err != nil {
		return sendFailed, fmt.Errorf("%s %s create request: %w", method, path, err)
	}
	resp, err := requester.Do(req)
	if err != nil {
		return sendFailed, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusAccepted:
		return sendDone, nil
	case isRefusedStatus(resp.StatusCode):
		return sendDropped, fmt.Errorf("%w: %s %s status code %d", errCollectorRefused, method, path, resp.StatusCode)
	default:
		return sendFailed, fmt.Errorf("%s %s unexpected status code: wanted %d got %d",
			method, path, http.StatusAccepted, resp.StatusCode)
	}
}

func (o *resInfoSent) filename() string {
	if o == nil {
		return ""
	}
	if len(o.cacheFile) == 0 {
		if o.path.IsZero() {
			return ""
		}
		flat := fmt.Sprintf("%s.%s.%s@%s.json", o.path.Namespace, o.path.Kind, o.path.Name, o.nodename)
		o.cacheFile = filepath.FromSlash(filepath.Join(rawconfig.CollectorResInfoSentDir(), flat))
	}
	return o.cacheFile
}

func (o *resInfoSent) write() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	sentTrace := o.filename()
	f, err := os.Create(sentTrace)
	if err != nil {
		if err1 := os.MkdirAll(filepath.Dir(sentTrace), 0755); err1 != nil {
			return errors.Join(err, err1)
		}
		if f, err = os.Create(sentTrace); err != nil {
			return err
		}
	}
	defer func() { _ = f.Close() }()
	return json.NewEncoder(f).Encode(o)
}

func (o *resInfoSent) read() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	f, err := os.Open(o.filename())
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(&o)
}

func (o *resInfoSent) drop() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	if err := os.Remove(o.filename()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
