package collector

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	objectConfigPost struct {
		Path string `json:"path"`

		Orchestrate string `json:"orchestrate"`

		Topology string `json:"topology"`

		Scope    []string `json:"scope"`
		DrpNode  string   `json:"drp_node"`
		DrpNodes []string `json:"drp_nodes"`

		FlexMin    int `json:"flex_min"`
		FlexMax    int `json:"flex_max"`
		FlexTarget int `json:"flex_target"`

		MonitoredResourceCount int `json:"monitored_resource_count"`

		App string `json:"app"`

		Env string `json:"env"`

		Comment string `json:"comment"`

		RawConfig []byte `json:"raw_config"`
	}

	// objectConfigSent describes object config sent to the collector db.
	// It is used to prevent send already sent configs to the collector and
	// is dumped to the filesystem <var>/collector/config_sent/<fqdn>.json to
	// populate sent cache after daemon restart.
	objectConfigSent struct {
		SentAt   time.Time `json:"sent_at"`
		Checksum string    `json:"csum"`

		path naming.Path

		// cacheFile is the file path to store objectConfigSent struct
		cacheFile string
	}
)

var (
	ErrZeroPath = errors.New("zero path")
)

// sendObjectConfigChange starts the send of a batch of the queued instance
// configs, unless one is in flight. The result comes back to the loop
// through sendResultC.
//
// A config sent, skipped or refused for good is dequeued. A send failing
// stops the batch: the config and the ones not tried stay queued for a
// later tick, as with the collector down the others would fail too, each
// after fetching its config.
func (t *T) sendObjectConfigChange() {
	if t.objectConfigSending || len(t.objectConfigToSend) == 0 || t.client == nil {
		return
	}
	jobs := make([]objectConfigJob, 0, min(len(t.objectConfigToSend), sendBatchMax))
	for p, v := range t.objectConfigToSend {
		if len(jobs) == sendBatchMax {
			break
		}
		job := objectConfigJob{path: p, queued: v, msg: v, sentChecksum: t.objectConfigSent[p].Checksum}
		if v == nil {
			// The collector says it lacks the config: send the one of
			// the node holding it.
			job.msg = t.createInstanceConfigUpdated(p)
		}
		jobs = append(jobs, job)
	}
	t.objectConfigSending = true
	epoch, ctx, requester, timeout, localhost := t.feedEpoch, t.ctx, t.client, t.postTimeout, t.localhost
	t.log.Tracef("send instance config: %s %s, %d items", http.MethodPost, oc3path.FeedObjectConfig, len(jobs))
	t.goSend(func() sendResult {
		r := objectConfigResult{epoch: epoch}
		for _, job := range jobs {
			item := sendObjectConfig(ctx, requester, timeout, localhost, job)
			r.items = append(r.items, item)
			if item.outcome == sendFailed {
				break
			}
		}
		return r
	})
}

// sendObjectConfig sends the instance config of job to the collector,
// unless the collector has it already.
func sendObjectConfig(ctx context.Context, requester requester, timeout time.Duration, localhost string, job objectConfigJob) (item objectConfigItem) {
	item.objectConfigJob = job
	checksum, b, err := objectConfigBody(ctx, localhost, job.path, job.msg)
	switch {
	case err != nil:
		item.outcome = sendDropped
		item.err = err
		return
	case len(b) == 0:
		item.outcome = sendDropped
		item.err = fmt.Errorf("empty body")
		return
	case checksum == job.sentChecksum:
		item.outcome = sendSkipped
		return
	}
	item.checksum = checksum
	method, path := http.MethodPost, oc3path.FeedObjectConfig
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := requester.NewRequestWithContext(ctx, method, path, bytes.NewBuffer(b))
	if err != nil {
		item.outcome, item.err = sendFailed, fmt.Errorf("%s %s create request: %w", method, path, err)
		return
	}
	resp, err := requester.Do(req)
	if err != nil {
		item.outcome, item.err = sendFailed, fmt.Errorf("%s %s: %w", method, path, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusAccepted:
		item.outcome, item.sentAt = sendDone, time.Now()
	case isRefusedStatus(resp.StatusCode):
		item.outcome, item.err = sendDropped, fmt.Errorf("%w: %s %s status code %d", errCollectorRefused, method, path, resp.StatusCode)
	default:
		item.outcome, item.err = sendFailed, fmt.Errorf("%s %s unexpected status code: %d", method, path, resp.StatusCode)
	}
	return
}

// apply applies the result of a batch of instance configs sent. A config
// replaced or deleted in flight stays as the loop left it: queued again for
// its newer version, or gone.
func (r objectConfigResult) apply(t *T) {
	t.objectConfigSending = false
	if r.epoch != t.feedEpoch {
		t.log.Debugf("drop the result of instance configs sent before a resync or a speaker change")
		return
	}
	var failErr error
	skipped := warnSeries{log: t.log}
	for _, item := range r.items {
		p := item.path
		switch item.outcome {
		case sendFailed:
			t.log.Debugf("post instance config %s: %s", p, item.err)
			failErr = item.err
			continue
		case sendDropped:
			// skip os.ErrNotExist, path may be deleted
			if !errors.Is(item.err, os.ErrNotExist) {
				skipped.add("skip send instance config %s: %s", p, item.err)
			}
		case sendSkipped:
			t.log.Debugf("skip already sent instance config %s with checksum %s", p, item.sentChecksum)
		}
		if v, ok := t.objectConfigToSend[p]; !ok || v != item.queued {
			continue
		}
		delete(t.objectConfigToSend, p)
		if item.outcome == sendDone {
			sent := objectConfigSent{path: p, Checksum: item.checksum, SentAt: item.sentAt}
			if err := sent.write(); err != nil {
				// the collector has it: not a send to retry
				t.log.Warnf("save the sent config flag for %s: %s", p, err)
			}
			t.objectConfigSent[p] = sent
		}
	}
	skipped.done("skip send instance config")
	t.objectConfigFailure.update(t.log, time.Now(), failErr, len(t.objectConfigToSend))
}

// objectConfigBody returns the feed object config body of the instance
// config v, read from the local config file when v is the localhost one,
// else fetched from the v node daemon, and the checksum of the config.
func objectConfigBody(ctx context.Context, localhost string, p naming.Path, v *msgbus.InstanceConfigUpdated) (checksum string, b []byte, err error) {
	if p.IsZero() {
		return "", []byte{}, fmt.Errorf("called from empty object path")
	}
	if v == nil {
		return "", []byte{}, fmt.Errorf("can't detect node holder for config")
	}
	config := v.Value

	pa := objectConfigPost{
		Path:  v.Path.String(),
		Scope: config.Scope,
	}
	// The resources, the orchestration and the flex settings are the
	// ones of an actor.
	if config.ActorConfig != nil {
		for _, r := range config.Resources {
			if r.IsMonitored {
				pa.MonitoredResourceCount++
			}
		}
		pa.Orchestrate = config.Orchestrate
		pa.Topology = config.Topology.String()
		pa.App = config.App
		pa.Env = config.Env
		if config.Flex != nil {
			pa.FlexMin = config.Flex.Min
			pa.FlexMax = config.Flex.Max
			pa.FlexTarget = config.Flex.Target
		}
	}

	// TODO: set DrpNode, DrpNodes, Comment, encap
	peer := v.Node

	if peer == localhost {
		if rawConfig, err := os.ReadFile(p.ConfigFile()); err != nil {
			return "", []byte{}, err
		} else {
			pa.RawConfig = rawConfig
		}
	} else if peer != "" {
		// retrieve from api
		var (
			port, addr string
		)
		if lsnr := daemonsubsystem.DataListener.Get(peer); lsnr != nil {
			if lsnr.Port != "" {
				port = lsnr.Port
			}
			if lsnr.Addr == "::" {
				addr = peer
			} else {
				addr = lsnr.Addr
			}
		}
		cli, err := client.New(
			client.WithURL(daemonenv.HTTPNodeAndPortURL(addr, port)),
			client.WithServerName(peer),
			client.WithBearerFunc(daemonauth.CreateNodeToken),
			client.WithCertificate(daemonenv.CertChainFile()),
		)
		if err != nil {
			return "", nil, fmt.Errorf("new client: %s", err)
		}
		params := &api.GetObjectConfigFileParams{}
		ctx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
		defer cancel()
		if resp, err := cli.GetObjectConfigFile(ctx, p.Namespace, p.Kind, p.Name, params); err != nil {
			return "", []byte{}, err
		} else if resp.StatusCode == http.StatusOK {
			if b, err := io.ReadAll(resp.Body); err != nil {
				return "", []byte{}, err
			} else {
				pa.RawConfig = b
			}
		} else {
			return "", nil, fmt.Errorf("retrieve remote config unexpected status code: %d", resp.StatusCode)
		}
	} else {
		return "", nil, fmt.Errorf("no node to fetch the config from")
	}
	checksum = fmt.Sprintf("%x", md5.Sum(pa.RawConfig))
	b, err = json.Marshal(pa)
	return checksum, b, err
}

// createInstanceConfigUpdated returns *msgbus.InstanceConfigUpdated from found
// instance config for p. If multiple value exists it will use localhost value
// unless more recent peer value exists with another checksum.
// nil is return when not found.
func (t *T) createInstanceConfigUpdated(p naming.Path) (v *msgbus.InstanceConfigUpdated) {
	configs := instance.ConfigData.GetByPath(p)
	if cfg, ok := configs[t.localhost]; ok {
		v = &msgbus.InstanceConfigUpdated{
			Path:  p,
			Node:  t.localhost,
			Value: *cfg,
		}
	}
	for nodename, cfg := range configs {
		if nodename == t.localhost {
			// already analysed
			continue
		}
		if v == nil || (cfg.UpdatedAt.After(v.Value.UpdatedAt) && cfg.Checksum != v.Value.Checksum) {
			// v is not yet set or found recent cfg with != checksum
			v = &msgbus.InstanceConfigUpdated{
				Path:  p,
				Node:  nodename,
				Value: *cfg,
			}
		}
	}
	return
}

func (o *objectConfigSent) filename() string {
	if o == nil {
		return ""
	}
	if len(o.cacheFile) == 0 {
		if o.path.IsZero() {
			return ""
		}
		flat := fmt.Sprintf("%s.%s.%s.json", o.path.Namespace, o.path.Kind, o.path.Name)
		o.cacheFile = filepath.FromSlash(filepath.Join(rawconfig.CollectorSentDir(), flat))
	}
	return o.cacheFile
}

func (o *objectConfigSent) write() error {
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

func (o *objectConfigSent) read() error {
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

func (o *objectConfigSent) drop() error {
	if o == nil || o.path.IsZero() {
		return ErrZeroPath
	}
	if err := os.Remove(o.filename()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
