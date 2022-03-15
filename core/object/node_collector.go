package object

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"opensvc.com/opensvc/core/collector"
	"opensvc.com/opensvc/util/key"
)

func (t Node) collectorClient() (*collector.Client, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	secret := t.config.GetString(key.Parse("node.uuid"))
	return collector.NewClient(s, secret)
}

func (t *Node) CollectorRestAPIURL() (*url.URL, error) {
	s := t.mergedConfig.GetString(key.Parse("node.dbopensvc"))
	return collector.RestURL(s)
}

func (t *Node) collectorRestAPIClient() *http.Client {
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
