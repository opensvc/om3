package cluster

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseAIAgentURL reads ai.agent_url: an https origin, without credentials,
// path, query or fragment, which the browsers running om3-webapp call.
func ParseAIAgentURL(value string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	switch {
	case u.Scheme != "https":
		return nil, fmt.Errorf("%q: an https origin is required", value)
	case u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "":
		return nil, fmt.Errorf("%q is no origin: give the scheme, the host and the port only", value)
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}
