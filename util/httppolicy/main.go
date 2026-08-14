package httppolicy

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"github.com/danwakefield/fnmatch"
)

type (
	T struct {
		// AllowedUrl defines the list of URL patterns that are explicitly
		// permitted by the policy regardless BlockedUrl setting.
		AllowedUrl []string

		// BlockedURL define the URLs patterns that are explicitly disallowed by the policy.
		// Empty value is treated the same as a []string{"*"} (all URLs are blocked
		// unless explicitly allowed).
		BlockedUrl []string

		// AllowedCIDR defines CIDR blocks explicitly allowed by the policy,
		// overriding any entries in BlockedCIDR.
		AllowedCIDR []string

		// BlockedCIDR defines a list of CIDR blocks that are explicitly disallowed
		// by the policy for incoming requests.
		BlockedCIDR []string
	}
)

// New creates and returns a new instance of T initialized with the provided whitelist URLs and blocked CIDRs.
func New(allowedURL, blockedURL, allowedCIDR, blockedCIDR []string) *T {
	return &T{
		AllowedUrl:  append([]string(nil), allowedURL...),
		BlockedUrl:  append([]string(nil), blockedURL...),
		AllowedCIDR: append([]string(nil), allowedCIDR...),
		BlockedCIDR: append([]string(nil), blockedCIDR...),
	}
}

// Check validates the given rawURL, resolves its IP, determines its port, and ensures it complies with set rules.
// The accepted ip, port is returned to prevent TOCTOU bugs.
func (t *T) Check(rawURL string) (ip net.IP, port string, err error) {
	if t == nil {
		return nil, "", fmt.Errorf("unexpected validator")
	}

	parsedURL, err := parseRequestURL(rawURL)
	if err != nil {
		return nil, "", err
	}

	host := parsedURL.Hostname()
	if host == "" {
		return nil, "", fmt.Errorf("empty host")
	}
	var ips []net.IP
	ips, err = net.LookupIP(host)
	if err != nil {
		return nil, "", err
	}

	if err := rejectURL(parsedURL, t.AllowedUrl, t.BlockedUrl); err != nil {
		return nil, "", fmt.Errorf("reject url %s: %w", parsedURL, err)
	}

	allowedNets := parseBlockedCIDRs(t.AllowedCIDR)
	blockedNets := parseBlockedCIDRs(t.BlockedCIDR)
	if ip, err = rejectIPs(ips, allowedNets, blockedNets); err != nil {
		return nil, "", fmt.Errorf("reject host %s ip %s: %w", host, ip, err)
	}
	port = parsedURL.Port()
	if port == "" {
		// set port based on the allowed URL scheme
		switch parsedURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return nil, "", fmt.Errorf("can't detect port for unsupported scheme %s", parsedURL.Scheme)
		}
	}
	return ip, port, nil
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

func rejectURL(requestURL *url.URL, allowed, blocked []string) error {
	host := requestURL.Hostname()
	scheme := requestURL.Scheme
	port := requestURL.Port()
	normalizedPath := path.Clean(requestURL.Path)

	// Verify exception from the allowed list
	for _, u := range allowed {
		allowedURL, err := url.Parse(u)
		if err != nil {
			continue
		}

		if isAllowedURLMatch(scheme, host, port, normalizedPath, allowedURL) {
			return nil
		}
	}

	// verify if match blocked url
	requestURL = &url.URL{
		Scheme: requestURL.Scheme,
		Host:   requestURL.Host,
		Path:   path.Clean(requestURL.Path),
	}

	if len(blocked) == 0 {
		blocked = []string{"*"}
	}
	requestURLString := requestURL.String()
	for _, u := range blocked {
		if fnmatch.Match(u, requestURLString, 0) {
			return fmt.Errorf("url %s is blocked by pattern %s and not in allowed list %s", requestURLString, u, allowed)
		}
	}

	return nil
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

func rejectIPs(ips []net.IP, allowedNet, blockedNets []*net.IPNet) (ip net.IP, err error) {
	if len(ips) == 0 {
		return ip, fmt.Errorf("empty ip list provided")
	}
	for _, ip = range ips {
		for _, blockedNet := range blockedNets {
			if blockedNet.Contains(ip) {
				// verify exceptions
				for _, allowed := range allowedNet {
					if allowed.Contains(ip) {
						return ip, nil
					}
				}
				return ip, fmt.Errorf("blocked net %s and not in exception net list %s", blockedNet, allowedNet)
			}
		}
	}
	// ips are no blocked, return fist ip from ip list
	return ips[0], nil
}
