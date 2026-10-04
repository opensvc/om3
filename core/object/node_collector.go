package object

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
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
	}

	CollectorProblem struct {
		text string
	}
)

var (
	defaultPostCollectorTimeout = 1 * time.Second
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
		method, path, resp.StatusCode, data.text)
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
	return &collector.Config{
		FeederUrl:    t.FeederUrl(),
		ServerUrl:    t.ServerUrl(),
		Timeout:      timeout,
		Insecure:     t.insecure,
		Password:     t.uuid,
		PingInterval: pingInterval,
		StatusDelay:  statusDelay,

		ActionBatch:      t.actionBatch,
		ActionLogTimeout: actionLogTimeout,
	}
}

func (t *Node) CollectorFeedClient() (*collector.Client, error) {
	cfg := t.CollectorRawConfig().AsConfig()
	if cfg.FeederUrl == "" {
		return nil, collector.ErrConfig
	}
	return cfg.NewFeedClient()
}

func (t Node) CollectorInitClient() (*collector.Client, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	secret := t.config.GetString(key.Parse("node.uuid"))
	return collector.NewInitClient(s, secret)
}

func (t Node) CollectorComplianceClient() (*collector.Client, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	secret := t.config.GetString(key.Parse("node.uuid"))
	return collector.NewComplianceClient(s, secret)
}

func (t *Node) CollectorRestAPIURL() (*url.URL, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	return collector.RestURL(s)
}

func (t *Node) Collector3RestAPIURL() (*url.URL, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	u, err := collector.RestURL(s)
	if err != nil {
		return u, err
	}
	u.Path = strings.Replace(u.Path, "/init/rest/api", "", 1)
	u.RawPath = strings.Replace(u.Path, "/init/rest/api", "", 1)
	return u, nil
}

func (t *Node) CollectorRestAPIClient() *http.Client {
	insecure := t.MergedConfig().GetBool(key.Parse("node.dbinsecure"))
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}
	return client
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
