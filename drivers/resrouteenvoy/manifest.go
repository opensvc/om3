package resrouteenvoy

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupRoute, "envoy")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		keywords.Keyword{
			Option:   "match_path",
			Attr:     "MatchPath",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/match_path"),
		},
		keywords.Keyword{
			Option:   "match_regex",
			Attr:     "MatchRegex",
			Scopable: true,
			Example:  "/b[io]t",
			Text:     keywords.NewText(fs, "text/kw/match_regex"),
		},
		keywords.Keyword{
			Option:   "match_prefix",
			Attr:     "MatchPrefix",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/match_prefix"),
		},
		keywords.Keyword{
			Option:    "match_case_sensitive",
			Attr:      "MatchCaseSensitive",
			Converter: converters.Bool,
			Default:   "true",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/match_case_sensitive"),
		},
		keywords.Keyword{
			Option:   "route_prefix_rewrite",
			Attr:     "RoutePrefixRewrite",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/route_prefix_rewrite"),
		},
		keywords.Keyword{
			Option:   "route_host_rewrite",
			Attr:     "RouteHostRewrite",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/route_host_rewrite"),
		},
		keywords.Keyword{
			Option:   "route_cluster_header",
			Attr:     "RouteClusterHeader",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/route_cluster_header"),
		},
		keywords.Keyword{
			Option:    "route_timeout",
			Attr:      "RouteTimeout",
			Converter: converters.Duration,
			Default:   "15s",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/route_timeout"),
		},
		keywords.Keyword{
			Option:   "redirect_host_redirect",
			Attr:     "RedirectHostRedirect",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_host_redirect"),
		},
		keywords.Keyword{
			Option:   "redirect_prefix_rewrite",
			Attr:     "RedirectPrefixRewrite",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_prefix_rewrite"),
		},
		keywords.Keyword{
			Option:   "redirect_path_redirect",
			Attr:     "RedirectPathRedirect",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_path_redirect"),
		},
		keywords.Keyword{
			Option:   "redirect_response_code",
			Attr:     "RedirectResponseCode",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_response_code"),
		},
		keywords.Keyword{
			Option:    "redirect_https_redirect",
			Attr:      "RedirectHTTPSRedirect",
			Converter: converters.Bool,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/redirect_https_redirect"),
		},
		keywords.Keyword{
			Option:    "redirect_strip_query",
			Attr:      "RedirectStripQuery",
			Converter: converters.Bool,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/redirect_strip_query"),
		},
		keywords.Keyword{
			Option:    "hash_policies",
			Attr:      "HashPolicies",
			Converter: converters.List,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/hash_policies"),
		},
	)
	return m
}
