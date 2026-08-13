package httppolicy

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
)

type (
	T struct {
		WhiteListUrls []string
		BlockedCIDRs  []string
	}
)

// New creates and returns a new instance of T initialized with the provided whitelist URLs and blocked CIDRs.
func New(whiteListUrls, blockedCIDRs []string) *T {
	return &T{
		WhiteListUrls: append([]string(nil), whiteListUrls...),
		BlockedCIDRs:  append([]string(nil), blockedCIDRs...),
	}
}

// Check checks if the provided raw URL is valid, matches whitelist criteria,
// and is not blocked by CIDR rules.
func (t *T) Check(rawURL string) error {
	if t == nil {
		return fmt.Errorf("unexpected validator")
	}

	parsedURL, err := parseRequestURL(rawURL)
	if err != nil {
		return err
	}

	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	if !isURLWhitelisted(parsedURL, t.WhiteListUrls) {
		return fmt.Errorf("url %s not in white list url %s", parsedURL, t.WhiteListUrls)
	}

	blockedNets := parseBlockedCIDRs(t.BlockedCIDRs)
	return rejectBlockedResolvedIPs(host, blockedNets)
}

func parseRequestURL(rawURL string) (*url.URL, error) {
	unescapedURL, err := url.PathUnescape(rawURL)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.ParseRequestURI(unescapedURL)
	if err != nil {
		return nil, err
	}
	if parsedURL == nil {
		return nil, fmt.Errorf("nil url")
	}

	return parsedURL, nil
}

func isURLWhitelisted(requestURL *url.URL, whitelistURLs []string) bool {
	host := requestURL.Hostname()
	scheme := requestURL.Scheme
	port := requestURL.Port()
	normalizedPath := path.Clean(requestURL.Path)

	for _, whitelistURL := range whitelistURLs {
		allowedURL, err := url.Parse(whitelistURL)
		if err != nil {
			continue
		}

		if isAllowedURLMatch(scheme, host, port, normalizedPath, allowedURL) {
			return true
		}
	}

	return false
}

func isAllowedURLMatch(scheme, host, port, normalizedPath string, allowedURL *url.URL) bool {
	if allowedURL.Scheme != "http" && allowedURL.Scheme != "https" {
		return false
	}

	allowedHost := allowedURL.Hostname()
	if allowedHost == "" {
		return false
	}

	if scheme != allowedURL.Scheme || host != allowedHost || port != allowedURL.Port() {
		return false
	}

	if allowedURL.Path == "" {
		return true
	}

	allowedPath := path.Clean(allowedURL.Path)
	return normalizedPath == allowedPath || strings.HasPrefix(normalizedPath, allowedPath+"/")
}

func parseBlockedCIDRs(blockedCIDRs []string) []*net.IPNet {
	blockedNets := make([]*net.IPNet, 0, len(blockedCIDRs))

	for _, cidrString := range blockedCIDRs {
		_, cidr, err := net.ParseCIDR(cidrString)
		if err != nil || cidr == nil {
			continue
		}

		blockedNets = append(blockedNets, cidr)
	}

	return blockedNets
}

func rejectBlockedResolvedIPs(host string, blockedNets []*net.IPNet) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		for _, blockedNet := range blockedNets {
			if blockedNet.Contains(ip) {
				return fmt.Errorf("ip %s from %s is blocked by CIDR %s", ip, host, blockedNet)
			}
		}
	}

	return nil
}
