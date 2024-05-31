package object

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/httphelper"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/requestfactory"
)

var (
	ErrNodeCollectorConfig       = errors.New("collector is not configured: empty configuration keyword node.dbopensvc")
	ErrNodeCollectorUnregistered = errors.New("this node is not registered. try 'om node register'")
)

func (t Node) CollectorFeedClient() (*collector.Client, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	secret := t.config.GetString(key.Parse("node.uuid"))
	return collector.NewFeedClient(s, secret)
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

// CollectorClient returns configured *httphelper.T for collector
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

	server, err := url.Parse(dbopensvc)
	if err != nil {
		return nil, err
	}
	// prepare default default header
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Authorization",
		"Basic "+base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+pass)))

	factory := requestfactory.New(server, header)

	cli := httphelper.NewHttpsClient(insecure)

	return httphelper.New(cli, factory), nil
}
