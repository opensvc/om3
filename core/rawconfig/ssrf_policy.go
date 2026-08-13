package rawconfig

import "strings"

var (
	defaultWhitelistURLs = []string{
		"https://raw.githubusercontent.com",
		"https://gitlab.com",
	}

	defaultBlockedCIDRs = []string{
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

func setSSRF(whiteListUrl, blockedCIDR string) {
	if whiteListUrl == "" {
		SSRFWhiteListUrl = defaultWhitelistURLs
	} else {
		SSRFWhiteListUrl = strings.Fields(whiteListUrl)
	}
	if blockedCIDR == "" {
		SSRFBlockedCIDR = defaultBlockedCIDRs
	} else {
		SSRFBlockedCIDR = strings.Fields(blockedCIDR)
	}
}
