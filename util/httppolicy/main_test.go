package httppolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name        string
		inputURL    string
		expectedErr bool
	}{
		{
			name:        "allows_https_github_relay_v3_config_path",
			inputURL:    "https://raw.githubusercontent.com/opensvc/opensvc_templates/refs/heads/main/relay-v3/relay-v3.conf",
			expectedErr: false,
		},
		{
			name:        "allows_https_github_host_only",
			inputURL:    "https://raw.githubusercontent.com",
			expectedErr: false,
		},
		{
			name:        "allows_https_github_host_with_trailing_slash",
			inputURL:    "https://raw.githubusercontent.com/",
			expectedErr: false,
		},
		{
			name:        "allows_https_gitlab_host_only",
			inputURL:    "https://gitlab.com",
			expectedErr: false,
		},
		{
			name:        "rejects_http_scheme",
			inputURL:    "http://raw.githubusercontent.com",
			expectedErr: true,
		},
		{
			name:        "rejects_file_scheme_with_empty_host",
			inputURL:    "file:///tmp/foo",
			expectedErr: true,
		},
		{
			name:        "rejects_disallowed_port_on_allowlisted_host_path",
			inputURL:    "https://github.com:666/opensvc/om3",
			expectedErr: true,
		},
		{
			name:        "allows_allowlisted_port_and_path",
			inputURL:    "https://github.com:8888/opensvc/om3",
			expectedErr: false,
		},
		{
			name:        "allows_subpath_under_allowlisted_path",
			inputURL:    "https://github.com:8888/opensvc/om3/bar",
			expectedErr: false,
		},
		{
			name:        "allows_normalized_path_within_allowlisted_path",
			inputURL:    "https://github.com:8888/opensvc/om3/../om3/bar",
			expectedErr: false,
		},
		{
			name:        "rejects_normalized_path_outside_allowlisted_path",
			inputURL:    "https://github.com:8888/opensvc/om3/../oc3/bar",
			expectedErr: true,
		},
		{
			name:        "rejects_escaped_dotdot_path_escape",
			inputURL:    "https://github.com:8888/opensvc/om3/%2e%2e/foo",
			expectedErr: true,
		},
		{
			name:        "rejects_path_prefix_without_separator",
			inputURL:    "https://github.com:8888/opensvc/om3foo",
			expectedErr: true,
		},
		{
			name:        "rejects_path_outside_allowlisted_prefix",
			inputURL:    "https://github.com:8888/opensvc/foo/bar",
			expectedErr: true,
		},
		{
			name:        "rejects_file_scheme_with_remote_style_host",
			inputURL:    "file://raw.githubusercontent.com/tmp/foo",
			expectedErr: true,
		},
		{
			name:        "rejects_unknown_scheme",
			inputURL:    "foo://raw.githubusercontent.com",
			expectedErr: true,
		},
		{
			name:        "rejects_disallowed_port_on_allowlisted_raw_github_host",
			inputURL:    "https://raw.githubusercontent.com:8888",
			expectedErr: true,
		},
		{
			name:        "rejects_non_allowlisted_host",
			inputURL:    "https://google.com",
			expectedErr: true,
		},
		{
			name:        "rejects_ipv4_loopback_resolved_from_localhost",
			inputURL:    "https://localhost",
			expectedErr: true,
		},
		{
			name:        "rejects_ipv6_loopback",
			inputURL:    "https://[::1]",
			expectedErr: true,
		},
		{
			name:        "rejects_private_ipv4_address",
			inputURL:    "https://10.0.0.1",
			expectedErr: true,
		},
		{
			name:        "rejects_unique_local_ipv6_address",
			inputURL:    "https://[fc00::1]",
			expectedErr: true,
		},
		{
			name:        "allows_public_ipv4_address",
			inputURL:    "https://8.8.8.8",
			expectedErr: false,
		},
		{
			name:        "rejects_malformed_url",
			inputURL:    ":/invalid-url",
			expectedErr: true,
		},
		{
			name:        "rejects_unresolvable_host",
			inputURL:    "https://invalid-host-lookup.opensvc.com",
			expectedErr: true,
		},
		{
			name:        "rejects_https_url_with_empty_host",
			inputURL:    "https://",
			expectedErr: true,
		},
		{
			name:        "rejects_empty_url",
			inputURL:    "",
			expectedErr: true,
		},
		{
			name:        "rejects_relative_path",
			inputURL:    "foo/bar",
			expectedErr: true,
		},
		{
			name:        "rejects_dot_relative_path",
			inputURL:    "./bar/foo",
			expectedErr: true,
		},
	}

	t.Run("raw config checker", func(t *testing.T) {
		v := New(rawconfig.SSRFWhiteListUrl, rawconfig.SSRFBlockedCIDR)

		v.WhiteListUrls = append(v.WhiteListUrls, "https://8.8.8.8", "https://github.com:8888/opensvc/om3")

		// to verify loopback ranges
		v.WhiteListUrls = append(v.WhiteListUrls, "https://localhost", "https://[::1]")

		// to verify invalid host lookup
		v.WhiteListUrls = append(v.WhiteListUrls, "https://invalid-host-lookup.opensvc.com")

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Check url: %s", tc.inputURL)
				err := v.Check(tc.inputURL)
				if tc.expectedErr {
					assert.Errorf(t, err, "expected error for %s", tc.inputURL)
					if err != nil {
						t.Logf("got expected error: %s", err)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("zero checker must reject all urls", func(t *testing.T) {
		v := New(nil, nil)

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Check url: %s", tc.inputURL)
				err := v.Check(tc.inputURL)
				assert.Errorf(t, err, "expected error for %s", tc.inputURL)
				if err != nil {
					t.Logf("got expected error: %s", err)
				}
			})
		}
	})

}
