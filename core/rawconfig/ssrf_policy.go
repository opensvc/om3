package rawconfig

import "strings"

var (
	defaultSSRFAllowedURL = []string{
		"https://raw.githubusercontent.com/opensvc/opensvc_templates",
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

func setSSRF(allowedURL, blockedURL, AllowedCIDR, blockedCIDR string) {
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
}
