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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		keywords.Keyword{
			Attr:     "MatchPath",
			Option:   "match_path",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/match_path"),
		},
		keywords.Keyword{
			Attr:     "MatchRegex",
			Example:  "/b[io]t",
			Option:   "match_regex",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/match_regex"),
		},
		keywords.Keyword{
			Attr:     "MatchPrefix",
			Option:   "match_prefix",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/match_prefix"),
		},
		keywords.Keyword{
			Attr:      "MatchCaseSensitive",
			Converter: converters.Bool,
			Default:   "true",
			Option:    "match_case_sensitive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/match_case_sensitive"),
		},
		keywords.Keyword{
			Attr:     "RoutePrefixRewrite",
			Option:   "route_prefix_rewrite",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/route_prefix_rewrite"),
		},
		keywords.Keyword{
			Attr:     "RouteHostRewrite",
			Option:   "route_host_rewrite",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/route_host_rewrite"),
		},
		keywords.Keyword{
			Attr:     "RouteClusterHeader",
			Option:   "route_cluster_header",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/route_cluster_header"),
		},
		keywords.Keyword{
			Attr:      "RouteTimeout",
			Converter: converters.Duration,
			Default:   "15s",
			Option:    "route_timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/route_timeout"),
		},
		keywords.Keyword{
			Attr:     "RedirectHostRedirect",
			Option:   "redirect_host_redirect",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_host_redirect"),
		},
		keywords.Keyword{
			Attr:     "RedirectPrefixRewrite",
			Option:   "redirect_prefix_rewrite",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_prefix_rewrite"),
		},
		keywords.Keyword{
			Attr:     "RedirectPathRedirect",
			Option:   "redirect_path_redirect",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_path_redirect"),
		},
		keywords.Keyword{
			Attr:     "RedirectResponseCode",
			Option:   "redirect_response_code",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/redirect_response_code"),
		},
		keywords.Keyword{
			Attr:      "RedirectHTTPSRedirect",
			Converter: converters.Bool,
			Option:    "redirect_https_redirect",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/redirect_https_redirect"),
		},
		keywords.Keyword{
			Attr:      "RedirectStripQuery",
			Converter: converters.Bool,
			Option:    "redirect_strip_query",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/redirect_strip_query"),
		},
		keywords.Keyword{
			Attr:      "HashPolicies",
			Converter: converters.List,
			Option:    "hash_policies",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/hash_policies"),
		},
	)
	return m
}
