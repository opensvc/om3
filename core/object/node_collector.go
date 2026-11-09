package object

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/httphelper"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	CollectorConfigRaw struct {
		collectorUrl string
		feederUrl    string
		serverUrl    string
		timeout      *time.Duration
		insecure     bool
		uuid         string
		pingInterval *time.Duration
		statusDelay  *time.Duration

		actionBatch      int
		actionLogTimeout *time.Duration

		speakerGracePeriod *time.Duration
	}

	CollectorProblem struct {
		Text string `json:"text"`
	}
)

// CollectorResponseStatusCheck verifies if the HTTP response status code matches any of the expected codes in `wanted`.
// If it doesn't match, attempts to decode the response body as a `CollectorProblem` and includes its details in the error.
func CollectorResponseStatusCheck(resp *http.Response, method, path string, wanted []int) error {
	if slices.Contains(wanted, resp.StatusCode) {
		return nil
	}
	var data CollectorProblem
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("unexpected response status code for %s %s: wanted %v got %d",
			method, path, wanted, resp.StatusCode)
	}
	return fmt.Errorf("%s %s: [%d]: %s",
		method, path, resp.StatusCode, data.Text)
}

func (t *Node) CollectorRawConfig() *CollectorConfigRaw {
	cfg := t.MergedConfig()
	return &CollectorConfigRaw{
		collectorUrl: cfg.GetString(collectorKey(cfg, &kwCollectorURL, &kwNodeCollector)),
		feederUrl:    cfg.GetString(collectorKey(cfg, &kwCollectorFeeder, &kwNodeCollectorFeeder)),
		serverUrl:    cfg.GetString(collectorKey(cfg, &kwCollectorServer, &kwNodeCollectorServer)),
		timeout:      cfg.GetDuration(collectorKey(cfg, &kwCollectorTimeout, &kwNodeCollectorTimeout)),
		insecure:     cfg.GetBool(key.Parse("node.dbinsecure")),
		pingInterval: cfg.GetDuration(collectorKey(cfg, &kwCollectorPingInterval, &kwNodeCollectorPingInterval)),
		statusDelay:  cfg.GetDuration(collectorKey(cfg, &kwCollectorStatusDelay, &kwNodeCollectorStatusDelay)),

		actionBatch:      cfg.GetInt(key.Parse(kwCollectorActionBatch.String())),
		actionLogTimeout: cfg.GetDuration(key.Parse(kwCollectorActionLogTimeout.String())),

		speakerGracePeriod: cfg.GetDuration(key.Parse(kwCollectorSpeakerGracePeriod.String())),

		// uuid is loaded from node.conf
		uuid: t.Config().GetString(key.Parse("node.uuid")),
	}
}

// collectorKey returns the key to read a collector setting from: the
// [collector] keyword kw when set, else the deprecated [node] keyword
// deprecated it replaces, or one of its aliases, when set, else kw for its
// default.
func collectorKey(cfg *xconfig.T, kw, deprecated *keywords.Keyword) key.T {
	k := key.New(kw.Section, kw.Option)
	if cfg.HasKey(k) {
		return k
	}
	for _, option := range append([]string{deprecated.Option}, deprecated.Aliases...) {
		if old := key.New(deprecated.Section, option); cfg.HasKey(old) {
			return old
		}
	}
	return k
}

func (t *CollectorConfigRaw) HasServerV3() bool {
	return t.collectorUrl != "" || t.serverUrl != ""
}

func (t *CollectorConfigRaw) FeederUrl() string {
	if t.feederUrl != "" {
		return t.feederUrl
	} else if t.collectorUrl != "" {
		return t.collectorUrl + "/feeder"
	} else {
		return ""
	}
}

func (t *CollectorConfigRaw) ServerUrl() string {
	if t.serverUrl != "" {
		return t.serverUrl
	} else if t.collectorUrl != "" {
		return t.collectorUrl + "/server"
	} else {
		return ""
	}
}

func (t *CollectorConfigRaw) AsConfig() *collector.Config {
	var timeout, pingInterval, statusDelay, actionLogTimeout time.Duration
	if t.timeout != nil {
		timeout = *t.timeout
	}
	if t.pingInterval != nil {
		pingInterval = *t.pingInterval
	}
	if t.statusDelay != nil {
		statusDelay = *t.statusDelay
	}
	if t.actionLogTimeout != nil {
		actionLogTimeout = *t.actionLogTimeout
	}
	var speakerGracePeriod time.Duration
	if t.speakerGracePeriod != nil {
		speakerGracePeriod = *t.speakerGracePeriod
	}
	return &collector.Config{
		FeederUrl:    t.FeederUrl(),
		ServerUrl:    t.ServerUrl(),
		Timeout:      collector.BoundTimeout(timeout),
		Insecure:     t.insecure,
		Password:     t.uuid,
		PingInterval: pingInterval,
		StatusDelay:  statusDelay,

		ActionBatch:      t.actionBatch,
		ActionLogTimeout: actionLogTimeout,

		SpeakerGracePeriod: collector.BoundSpeakerGracePeriod(speakerGracePeriod),
	}
}

// collectorRequestContext returns ctx bounded by collector.timeout, for a
// request to the collector, and that timeout.
func (t *Node) collectorRequestContext(ctx context.Context) (context.Context, context.CancelFunc, time.Duration) {
	timeout := t.CollectorRawConfig().AsConfig().Timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return ctx, cancel, timeout
}

// collectorRequestError returns the error err of the request method path to
// the collector, saying the request timed out after timeout, the value of
// collector.timeout, when it did.
func collectorRequestError(err error, method, path string, timeout time.Duration) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("collector %s %s: no answer within %s (collector.timeout): %w", method, path, timeout, err)
	}
	return fmt.Errorf("collector %s %s: %w", method, path, err)
}

// CollectorServerAPIURL returns the url of the api of the oc3 server, which
// the {collector_api} reference names.
func (t *Node) CollectorServerAPIURL() (*url.URL, error) {
	s := t.CollectorRawConfig().ServerUrl()
	if s == "" {
		return nil, collector.ErrConfig
	}
	u, err := collector.BaseURL(s)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api"
	u.RawPath = ""
	return u, nil
}

// CollectorFeeder returns new collector feeder client from config
func (t *Node) CollectorFeeder() (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.FeederUrl == "" {
		return nil, collector.ErrConfig
	} else if cfg.Password == "" {
		return nil, collector.ErrUnregistered
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+cfg.Password))
	return collector.NewRequester(cfg.FeederUrl, auth, cfg.Insecure)
}

// CollectorServer returns new collector server client from config
func (t *Node) CollectorServer() (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.ServerUrl == "" {
		return nil, collector.ErrConfig
	} else if cfg.Password == "" {
		return nil, collector.ErrUnregistered
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+cfg.Password))
	return collector.NewRequester(cfg.ServerUrl, auth, cfg.Insecure)
}

// CollectorServer returns new collector server client from config
func (t *Node) CollectorServerWithAuth(auth string) (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.ServerUrl == "" {
		return nil, collector.ErrConfig
	}

	return collector.NewRequester(cfg.ServerUrl, auth, cfg.Insecure)
}

// CollectorServer returns new collector server client from config
func (t *Node) CollectorServerWithoutAuth() (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.ServerUrl == "" {
		return nil, collector.ErrConfig
	}

	return collector.NewRequester(cfg.ServerUrl, "", cfg.Insecure)
}
