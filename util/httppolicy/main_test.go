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
			inputURL:    "https://www.github.com",
			expectedErr: false,
		},
		{
			name:        "allows_https_github_host_with_trailing_slash",
			inputURL:    "https://www.github.com/",
			expectedErr: false,
		},
		{
			name:        "rejects_http_scheme",
			inputURL:    "http://raw.githubusercontent.com/opensvc/opensvc_templates/refs/heads/main/relay-v3/relay-v3.conf",
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
			name:        "allows_exception_url_with_port_and_path",
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
			name:        "block_cidr",
			inputURL:    "https://127.0.0.3/foo",
			expectedErr: true,
		},
		{
			name:        "block_cidr_v4_mapped_v6",
			inputURL:    "https://[::ffff:127.0.0.3]/foo",
			expectedErr: true,
		},
		{
			name:        "allow_cidr_exception_v4_mapped_v6",
			inputURL:    "https://[::ffff:127.0.0.2]/foo",
			expectedErr: false,
		},
		{
			name:        "allow_cidr_exception",
			inputURL:    "https://127.0.0.2/foo",
			expectedErr: false,
		},
		{
			name:        "rejects_ipv6_loopback",
			inputURL:    "https://[::1]",
			expectedErr: true,
		},
		{
			name:        "reject_ula_ipv6_cidr",
			inputURL:    "https://[fd7a:115c:a1e0:ab12:4843:cd96:626b:626b]",
			expectedErr: true,
		},
		{
			name:        "allow_ula_ipv6_cidr_exception",
			inputURL:    "https://[fd7a:115c:a1e0:ab12:4843:cd96:626b:430b]",
			expectedErr: false,
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
		v := New(rawconfig.SSRFAllowedURL, rawconfig.SSRFBlockedURL, rawconfig.SSRFAllowedCIDR, rawconfig.SSRFBlockedCIDR)

		v.AllowedUrl = append(v.AllowedUrl, "https://8.8.8.8", "https://github.com:8888/opensvc/om3", "https://www.github.com")

		// to verify loopback ranges
		v.AllowedUrl = append(v.AllowedUrl, "https://localhost", "https://[::1]")

		// to verify invalid host lookup
		v.AllowedUrl = append(v.AllowedUrl, "https://invalid-host-lookup.opensvc.com")

		v.AllowedUrl = append(v.AllowedUrl, "https://127.0.0.2/foo", "https://127.0.0.3/foo")
		v.AllowedUrl = append(v.AllowedUrl, "https://[::ffff:127.0.0.2]/foo", "https://[::ffff:127.0.0.3]/foo")
		v.AllowedCIDR = append(v.AllowedCIDR, "127.0.0.10/32", "127.0.0.2/32")

		v.AllowedUrl = append(v.AllowedUrl,
			"https://[fd7a:115c:a1e0:ab12:4843:cd96:626b:626b]", // should be rejected by ula rule
			"https://[fd7a:115c:a1e0:ab12:4843:cd96:626b:430b]", // should be accepted in AllowedCIDR
		)
		v.AllowedCIDR = append(v.AllowedCIDR, "fd7a:115c:a1e0:ab12:4843:cd96:626b:430b/128")

		t.Logf("policy AllowedCIDR: %s", v.AllowedCIDR)
		t.Logf("policy BlockedUrl: %s", v.BlockedUrl)
		t.Logf("policy BlockedCIDR: %s", v.BlockedCIDR)
		t.Logf("policy AllowedCIDR: %s", v.AllowedCIDR)
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Check url: %s", tc.inputURL)
				ip, port, err := v.Check(tc.inputURL)
				if tc.expectedErr {
					assert.Errorf(t, err, "expected error for %s", tc.inputURL)
					if err != nil {
						t.Logf("Check url error: %s", err)
					}
				} else {
					t.Logf("url detected ip %s, port %s", ip, port)
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("zero checker must reject all urls", func(t *testing.T) {
		v := New(nil, nil, nil, nil)

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Check url: %s", tc.inputURL)
				ip, port, err := v.Check(tc.inputURL)
				assert.Errorf(t, err, "Check url for %s", tc.inputURL)
				t.Logf("url detected ip %s, port %s", ip, port)
				if err != nil {
					t.Logf("got expected error: %s", err)
				}
			})
		}
	})

}
