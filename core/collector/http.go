package collector

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/httphelper"
	"github.com/opensvc/om3/v3/util/requestfactory"
)

func NewRequester(collectorURL string, auth string, insecure bool) (*httphelper.T, error) {
	server, err := BaseURL(collectorURL)
	if err != nil {
		return nil, err
	}
	// prepare default default header
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	if auth != "" {
		header.Set("Authorization", auth)
	}
	factory := requestfactory.New(server, header)

	cli := httphelper.NewHttpsClient(insecure)

	return httphelper.New(cli, factory), nil
}

// BaseURL returns the url s names, sanitized: no credential, query or
// fragment, https when no scheme is given, and a host when s is a host name
// alone.
func BaseURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// sanitize
	u.Opaque = ""
	u.User = nil
	u.ForceQuery = false
	u.RawQuery = ""
	u.Fragment = ""
	u.RawFragment = ""

	// default scheme is https
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	// A host name alone is a host, where url.Parse sees a path.
	if u.Host == "" && !strings.Contains(u.Path, "/") {
		u.Host = u.Path
		u.Path = ""
		u.RawPath = ""
	}

	return u, nil
}

// NewFeedRequester returns a client of the oc3 feeder, authenticated as the
// node.
func (c *Config) NewFeedRequester() (*httphelper.T, error) {
	if c.FeederUrl == "" {
		return nil, ErrConfig
	} else if c.Password == "" {
		return nil, ErrUnregistered
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(hostname.Hostname()+":"+c.Password))
	return NewRequester(c.FeederUrl, auth, c.Insecure)
}
