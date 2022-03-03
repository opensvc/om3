package object

import (
	"net/url"

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
