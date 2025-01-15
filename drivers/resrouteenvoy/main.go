package resrouteenvoy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
)

type (
	T struct {
		resource.T
		MatchPath             string         `json:"match_path,omitempty"`
		MatchRegex            string         `json:"match_regex,omitempty"`
		MatchPrefix           string         `json:"match_prefix,omitempty"`
		MatchCaseSensitive    bool           `json:"match_case_sensitive,omitempty"`
		RoutePrefixRewrite    string         `json:"route_prefix_rewrite,omitempty"`
		RouteHostRewrite      string         `json:"route_host_rewrite,omitempty"`
		RouteClusterHeader    string         `json:"route_cluster_header,omitempty"`
		RouteTimeout          *time.Duration `json:"route_timeout,omitempty"`
		RedirectHostRedirect  string         `json:"redirect_host_redirect,omitempty"`
		RedirectPrefixRewrite string         `json:"redirect_prefix_rewrite,omitempty"`
		RedirectPathRedirect  string         `json:"redirect_path_redirect,omitempty"`
		RedirectResponseCode  string         `json:"redirect_response_code,omitempty"`
		RedirectHTTPSRedirect bool           `json:"redirect_https_redirect,omitempty"`
		RedirectStripQuery    bool           `json:"redirect_strip_query,omitempty"`
		HashPolicies          []string       `json:"hash_policies,omitempty"`
	}
)

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	var match string
	l := make([]string, 0)
	r := make([]string, 0)
	w := make([]string, 0)

	if t.MatchPath != "" {
		match = t.MatchPath
	} else if t.MatchRegex != "" {
		match = t.MatchRegex
	} else if t.MatchPrefix != "" {
		match = t.MatchPrefix
	} else {
		match = "nothing"
	}
	l = append(l, fmt.Sprintf("match %s", match))

	if t.RedirectHostRedirect != "" {
		r = append(r, "host "+t.RedirectHostRedirect)
	}
	if t.RedirectPathRedirect != "" {
		r = append(r, "path "+t.RedirectPathRedirect)
	}
	if t.RedirectPrefixRewrite != "" {
		r = append(r, "prefix "+t.RedirectPrefixRewrite)
	}
	if t.RedirectResponseCode != "" {
		r = append(r, "rcode "+t.RedirectResponseCode)
	}
	if len(r) > 0 {
		l = append(l, "redirect to "+strings.Join(r, " "))
	}

	if t.RouteHostRewrite != "" {
		w = append(w, "host rewrite "+t.RouteHostRewrite)
	}
	if t.RoutePrefixRewrite != "" {
		w = append(w, "prefix rewrite "+t.RoutePrefixRewrite)
	}
	if t.RouteClusterHeader != "" {
		w = append(w, "cluster header "+t.RouteClusterHeader)
	}
	if len(w) > 0 {
		l = append(l, strings.Join(w, " "))
	}
	return strings.Join(l, " ")
}

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) Start(ctx context.Context) error {
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(_ context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	data["match_path"] = t.MatchPath
	data["match_regex"] = t.MatchRegex
	data["match_prefix"] = t.MatchPrefix
	data["match_case_sensitive"] = t.MatchCaseSensitive
	data["route_prefix_rewrite"] = t.RoutePrefixRewrite
	data["route_host_rewrite"] = t.RouteHostRewrite
	data["route_cluster_header"] = t.RouteClusterHeader
	data["route_timeout"] = t.RouteTimeout
	data["redirect_host_redirect"] = t.RedirectHostRedirect
	data["redirect_prefix_rewrite"] = t.RedirectPrefixRewrite
	data["redirect_path_redirect"] = t.RedirectPathRedirect
	data["redirect_response_code"] = t.RedirectResponseCode
	data["redirect_https_redirect"] = t.RedirectHTTPSRedirect
	data["redirect_strip_query"] = t.RedirectStripQuery
	data["hash_policies"] = t.HashPolicies
	return data
}
