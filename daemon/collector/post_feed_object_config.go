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
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
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

func (t *T) sendObjectConfigChange() (err error) {
	t.log.Tracef("sendObjectConfigChange")
	for p, v := range t.objectConfigToSend {
		checksum, b, err := t.asPostFeedObjectConfigBody(p, v)
		if err != nil {
			// skip os.ErrNotExist, path may be deleted
			if !errors.Is(err, os.ErrNotExist) {
				t.log.Warnf("skip send instance config %s: %s", p, err)
			}
			continue
		} else if len(b) == 0 {
			t.log.Warnf("skip send instance config %s: empty body", p)
			continue
		}

		sent := objectConfigSent{path: p}
		if err1 := sent.read(); err1 != nil && !errors.Is(err1, fs.ErrNotExist) {
			t.log.Warnf("removing corrupted sent config flag for %s: %s", p, err1)
			if err1 := sent.drop(); err1 != nil && !errors.Is(err1, fs.ErrNotExist) {
				t.log.Errorf("remove corrupted sent config flag for %s: %s", p, err1)
			}
			continue
		}

		if checksum == sent.Checksum {
			t.log.Infof("skip already sent instance config %s with checksum %s", p, checksum)
			continue
		}
		if err := t.doPostObjectConfig(checksum, b, p); err != nil {
			t.log.Warnf("post instance config %s: %s", p, err)
			continue
		}
	}
	t.objectConfigToSend = make(map[naming.Path]*msgbus.InstanceConfigUpdated)
	return
}

func (t *T) asPostFeedObjectConfigBody(p naming.Path, v *msgbus.InstanceConfigUpdated) (checksum string, b []byte, err error) {
	if p.IsZero() {
		return "", []byte{}, fmt.Errorf("called from empty object path")
	}
	if v == nil {
		// lost config has been detected from collector db
		v = t.createInstanceConfigUpdated(p)
		if v == nil {
			return "", []byte{}, fmt.Errorf("can't detect node holder for config")
		}
	}
	config := v.Value

	monResCount := 0
	for _, r := range config.Resources {
		if r.IsMonitored {
			monResCount++
		}
	}

	pa := objectConfigPost{
		Path:                   v.Path.String(),
		MonitoredResourceCount: monResCount,
		Scope:                  config.Scope,
	}
	if config.ActorConfig != nil {
		pa.Orchestrate = config.Orchestrate
		pa.Topology = config.Topology.String()
		pa.App = config.App
		pa.Env = config.Env
	}
	if config.Flex != nil {
		pa.FlexMin = config.Flex.Min
		pa.FlexMax = config.Flex.Max
		pa.FlexTarget = config.Flex.Target
	}

	// TODO: set DrpNode, DrpNodes, Comment, encap
	peer := v.Node

	if peer == t.localhost {
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
		t.log.Tracef("use client url from %s and %s: %s", addr, port, daemonenv.HTTPNodeAndPortURL(addr, port))
		cli, err := client.New(
			client.WithURL(daemonenv.HTTPNodeAndPortURL(addr, port)),
			client.WithUsername(hostname.Hostname()),
			client.WithPassword(cluster.ConfigData.Get().Secret()),
			client.WithCertificate(daemonenv.CertChainFile()),
		)
		if err != nil {
			return "", nil, fmt.Errorf("new client: %s", err)
		}
		t.log.Infof("retrieve remote config %s@%s", p, peer)
		if resp, err := cli.GetObjectConfigFile(t.ctx, p.Namespace, p.Kind, p.Name); err != nil {
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
		t.log.Infof("no peer node to fetch config")
		return "", nil, nil
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

func (t *T) doPostObjectConfig(checksum string, b []byte, p naming.Path) error {
	if t.client == nil {
		return nil
	}
	var (
		req  *http.Request
		resp *http.Response

		err error

		method = http.MethodPost
		path   = oc3path.FeedObjectConfig
	)

	ctx, cancel := context.WithTimeout(t.ctx, defaultPostMaxDuration)
	defer cancel()

	req, err = t.client.NewRequestWithContext(ctx, method, path, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	t.log.Infof("%s %s %s", method, path, p)
	resp, err = t.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %s", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusAccepted:
		t.log.Tracef("%s %s %s status code %d", method, path, p, resp.StatusCode)
		sent := objectConfigSent{path: p, Checksum: checksum, SentAt: time.Now()}
		if err := sent.write(); err != nil {
			return err
		}
		t.objectConfigSent[p] = sent
		return nil
	default:
		return fmt.Errorf("%s %s unexpected status code: %d", method, path, resp.StatusCode)
	}
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
