package resrouteenvoy

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupRoute, "envoy")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "match_path",
			Attr:     "MatchPath",
			Scopable: true,
			Text:     "If specified, the route is an exact path rule meaning that the path must exactly match the :path header once the query string is removed. Precisely one of prefix, path, regex must be set.",
		},
		{
			Option:   "match_regex",
			Attr:     "MatchRegex",
			Scopable: true,
			Text:     "If specified, the route is a regular expression rule meaning that the regex must match the :path header once the query string is removed. The entire path (without the query string) must match the regex. The rule will not match if only a subsequence of the :path header matches the regex.",
			Example:  "/b[io]t",
		},
		{
			Option:   "match_prefix",
			Attr:     "MatchPrefix",
			Scopable: true,
			Text:     "If specified, the route is a prefix rule meaning that the prefix must match the beginning of the :path header. Precisely one of prefix, path, regex must be set.",
		},
		{
			Option:    "match_case_sensitive",
			Attr:      "MatchCaseSensitive",
			Converter: converters.Bool,
			Default:   "true",
			Scopable:  true,
			Text:      "Indicates that prefix/path matching should be case sensitive. The default is ``true``.",
		},
		{
			Option:   "route_prefix_rewrite",
			Attr:     "RoutePrefixRewrite",
			Scopable: true,
			Text:     "The string replacing the url path prefix if matching.",
		},
		{
			Option:   "route_host_rewrite",
			Attr:     "RouteHostRewrite",
			Scopable: true,
			Text:     "Indicates that during forwarding, the host header will be swapped with this value.",
		},
		{
			Option:   "route_cluster_header",
			Attr:     "RouteClusterHeader",
			Scopable: true,
			Text:     "If the route is not a redirect (host_redirect and/or path_redirect is not specified), one of cluster, cluster_header, or weighted_clusters must be specified. When cluster_header is specified, Envoy will determine the cluster to route to by reading the value of the HTTP header named by cluster_header from the request headers. If the header is not found or the referenced cluster does not exist, Envoy will return a 404 response.",
		},
		{
			Option:    "route_timeout",
			Attr:      "RouteTimeout",
			Converter: converters.Duration,
			Default:   "15s",
			Scopable:  true,
			Text:      "Specifies the timeout for the route. If not specified. Note that this timeout includes all retries.",
		},
		{
			Option:   "redirect_host_redirect",
			Attr:     "RedirectHostRedirect",
			Scopable: true,
			Text:     "The host portion of the URL will be swapped with this value.",
		},
		{
			Option:   "redirect_prefix_rewrite",
			Attr:     "RedirectPrefixRewrite",
			Scopable: true,
			Text:     "Indicates that during redirection, the matched prefix (or path) should be swapped with this value. This option allows redirect URLs be dynamically created based on the request.",
		},
		{
			Option:   "redirect_path_redirect",
			Attr:     "RedirectPathRedirect",
			Scopable: true,
			Text:     "Indicates that the route is a redirect rule. If there is a match, a 301 redirect response will be sent which swaps the path portion of the URL with this value. host_redirect can also be specified along with this option.",
		},
		{
			Option:   "redirect_response_code",
			Attr:     "RedirectResponseCode",
			Scopable: true,
			Text:     "The HTTP status code to use in the redirect response. The default response code is MOVED_PERMANENTLY (301).",
		},
		{
			Option:    "redirect_https_redirect",
			Attr:      "RedirectHTTPSRedirect",
			Converter: converters.Bool,
			Scopable:  true,
			Text:      "The scheme portion of the URL will be swapped with 'https'.",
		},
		{
			Option:    "redirect_strip_query",
			Attr:      "RedirectStripQuery",
			Converter: converters.Bool,
			Scopable:  true,
			Text:      "Indicates that during redirection, the query portion of the URL will be removed.",
		},
		{
			Option:    "hash_policies",
			Attr:      "HashPolicies",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The list of hash policy resource ids for the route. Honored if lb_policy is set to ring_hash or maglev.",
		},
	}...)
	return m
}
