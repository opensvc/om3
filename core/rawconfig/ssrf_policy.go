package rawconfig

import "strings"

var (
	defaultSSRFAllowedURL = []string{
		"https://raw.githubusercontent.com/opensvc/opensvc_templates/*",
	}

	defaultSSRFBlockedURL = []string{
		"*",
	}

	defaultSSRFAllowedCIDR = []string{}

	defaultSSRFBlockedCIDR = []string{
		"127.0.0.0/8",     // RFC 1122 loopback
		"10.0.0.0/8",      // RFC 1918 private
		"172.16.0.0/12",   // RFC 1918 private
		"192.168.0.0/16",  // RFC 1918 private
		"169.254.0.0/16",  // link local
		"192.0.2.0/24",    // RFC 5737 private TEST-NET-1
		"198.51.100.0/24", // RFC 5737 private TEST-NET-2
		"203.0.113.0/24",  // RFC 5737 private TEST-NET-3

		// IPV6
		"::1/128",   // RFC 4291 loopback
		"fe80::/10", // link-local
		"fc00::/7",  // RFC 4193 Unique Local IPv6 Unicast Addresses
		"ff00::/8",  // reserved
	}
)

// setSSRF configures SSRF protection settings, including allowed/blocked URLs, CIDR ranges, and redirect behavior.
// Default is:
// OSVC_SSRF_ALLOWED_URL = https://raw.githubusercontent.com/opensvc/opensvc_templates/*
// OSVC_SSRF_BLOCKED_URL = *
// OSVC_SSRF_BLOCKED_CIDR = 127.0.0.0/8 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 169.254.0.0/16 192.0.2.0/24 198.51.100.0/24 203.0.113.0/24 ::1/128 fe80::/10 fc00::/7 ff00::/8
// OSVC_SSRF_ALLOWED_CIDR =
// OSVC_SSRF_ENABLE_REDIRECTS = false
func setSSRF(env map[string]string) {

	SSRFAllowedURL = getSSRFValue(env, "OSVC_SSRF_ALLOWED_URL", defaultSSRFAllowedURL)
	SSRFBlockedURL = getSSRFValue(env, "OSVC_SSRF_BLOCKED_URL", defaultSSRFBlockedURL)
	SSRFAllowedCIDR = getSSRFValue(env, "OSVC_SSRF_ALLOWED_CIDR", defaultSSRFAllowedCIDR)
	SSRFBlockedCIDR = getSSRFValue(env, "OSVC_SSRF_BLOCKED_CIDR", defaultSSRFBlockedCIDR)
	if enableRedirects, _ := env["OSVC_SSRF_ENABLE_REDIRECTS"]; enableRedirects == "true" {
		SSRFEnableRedirects = true
	} else {
		SSRFEnableRedirects = false
	}
}

func getSSRFValue(env map[string]string, varName string, defaultValue []string) []string {
	if v, ok := env[varName]; !ok {
		return append([]string{}, defaultValue...)
	} else if v == "" {
		return []string{}
	} else {
		return append([]string{}, strings.Fields(v)...)
	}
}
