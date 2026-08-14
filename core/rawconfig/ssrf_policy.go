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
		// RFC 1918 private address ranges
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",

		// RFC 4193 Unique Local IPv6 Unicast Addresses
		"fc00::/7",

		// Loopback ranges
		"127.0.0.0/8", // RFC 1122
		"::1/128",     // RFC 4291
	}
)

// setSSRF configures SSRF protection settings, including allowed/blocked URLs, CIDR ranges, and redirect behavior.
// Default is:
// OSVC_SSRF_ALLOWED_URL = https://raw.githubusercontent.com/opensvc/opensvc_templates
// OSVC_SSRF_BLOCKED_URL = *
// OSVC_SSRF_BLOCKED_CIDR = 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 fc00::/7 127.0.0.0/8 ::1/128
// OSVC_SSRF_ALLOWED_CIDR =
// OSVC_SSRF_ENABLE_REDIRECTS = false
func setSSRF(allowedURL, blockedURL, AllowedCIDR, blockedCIDR, enableRedirects string) {
	if allowedURL == "" {
		SSRFAllowedURL = defaultSSRFAllowedURL
	} else {
		SSRFAllowedURL = strings.Fields(allowedURL)
	}
	if blockedURL == "" {
		SSRFBlockedURL = defaultSSRFBlockedURL
	} else {
		SSRFBlockedURL = strings.Fields(blockedURL)
	}
	if AllowedCIDR == "" {
		SSRFAllowedCIDR = defaultSSRFAllowedCIDR
	} else {
		SSRFAllowedCIDR = strings.Fields(AllowedCIDR)
	}
	if blockedCIDR == "" {
		SSRFBlockedCIDR = defaultSSRFBlockedCIDR
	} else {
		SSRFBlockedCIDR = strings.Fields(blockedCIDR)
	}

	// SSRFEnableRedirects is always false by default unless enableRedirects == "true"
	SSRFEnableRedirects = false
	if enableRedirects == "true" {
		SSRFEnableRedirects = true
	}
}
