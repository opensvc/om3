package object

import (
	"crypto/tls"
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
	}

	CollectorConfig struct {
		FeederUrl string        `json:"feeder_url"`
		ServerUrl string        `json:"server_url"`
		Timeout   time.Duration `json:"timeout"`
		Insecure  bool          `json:"insecure"`

		// private field
		password string
	}

	CollectorProblem struct {
		Detail string `json:"detail"`
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
)

var (
	ErrNodeCollectorConfig       = errors.New("collector is not configured: empty configuration keyword node.dbopensvc")
	ErrNodeCollectorUnregistered = errors.New("this node is not registered. try 'om node register'")

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
	return fmt.Errorf("unexpected response status code for %s %s: wanted %v got %d %s: %s",
		method, path, wanted, resp.StatusCode, data.Title, data.Detail)
}

func (t *Node) CollectorRawConfig() *CollectorConfigRaw {
	cfg := t.MergedConfig()
	return &CollectorConfigRaw{
		collectorUrl: cfg.GetString(key.Parse("node.collector")),
		feederUrl:    cfg.GetString(key.Parse("node.collector_feeder")),
		serverUrl:    cfg.GetString(key.Parse("node.collector_server")),
		timeout:      cfg.GetDuration(key.Parse("node.collector_timeout")),
		insecure:     cfg.GetBool(key.Parse("node.dbinsecure")),

		// uuid is loaded from node.conf
		uuid: t.Config().GetString(key.Parse("node.uuid")),
	}
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

func (t *CollectorConfigRaw) AsConfig() *CollectorConfig {
	var timeout time.Duration
	if t.timeout != nil {
		timeout = *t.timeout
	}
	return &CollectorConfig{
		FeederUrl: t.FeederUrl(),
		ServerUrl: t.ServerUrl(),
		Timeout:   timeout,
		Insecure:  t.insecure,
		password:  t.uuid,
	}
}

func (t *Node) CollectorFeedClient() (*collector.Client, error) {
	collectorCfg := t.CollectorRawConfig().AsConfig()
	return collector.NewFeedClient(collectorCfg.FeederUrl, collectorCfg.password)
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

// CollectorClient returns new client collector from config
func (t *Node) CollectorClient() (*httphelper.T, error) {
	dbopensvc := t.MergedConfig().GetString(key.Parse("node.dbopensvc"))
	insecure := t.MergedConfig().GetBool(key.Parse("node.dbinsecure"))
	pass := t.MergedConfig().GetString(key.Parse("node.uuid"))

	if dbopensvc == "" || dbopensvc == "none" {
		return nil, ErrNodeCollectorConfig
	}

	if dbopensvc != "" && pass == "" {
		return nil, ErrNodeCollectorUnregistered
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+pass))
	return collector.NewRequester(dbopensvc, auth, insecure)
}

// CollectorFeeder returns new collector feeder client from config
func (t *Node) CollectorFeeder() (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.FeederUrl == "" {
		return nil, ErrNodeCollectorConfig
	} else if cfg.password == "" {
		return nil, ErrNodeCollectorUnregistered
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+cfg.password))
	return collector.NewRequester(cfg.FeederUrl, auth, cfg.Insecure)
}

// CollectorServer returns new collector server client from config
func (t *Node) CollectorServer() (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.ServerUrl == "" {
		return nil, ErrNodeCollectorConfig
	} else if cfg.password == "" {
		return nil, ErrNodeCollectorUnregistered
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+cfg.password))
	return collector.NewRequester(cfg.ServerUrl, auth, cfg.Insecure)
}

// CollectorServer returns new collector server client from config
func (t *Node) CollectorServerWithAuth(auth string) (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.ServerUrl == "" {
		return nil, ErrNodeCollectorConfig
	}

	return collector.NewRequester(cfg.ServerUrl, auth, cfg.Insecure)
}

// CollectorServer returns new collector server client from config
func (t *Node) CollectorServerWithoutAuth() (*httphelper.T, error) {
	cfg := t.CollectorRawConfig().AsConfig()

	if cfg.ServerUrl == "" {
		return nil, ErrNodeCollectorConfig
	}

	return collector.NewRequester(cfg.ServerUrl, "", cfg.Insecure)
}
