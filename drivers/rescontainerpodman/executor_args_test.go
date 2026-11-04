package rescontainerpodman

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
)

func TestExecutorArg_RunArgsBase(t *testing.T) {
	// The arguments without the unprivileged ports sysctl, which
	// TestUnprivilegedPortsSysctlWanted tests.
	prev := unprivilegedPortsSysctlFile
	unprivilegedPortsSysctlFile = "/nonexistent"
	defer func() { unprivilegedPortsSysctlFile = prev }()

	p := naming.Path{Name: "foo", Kind: naming.KindSvc}

	cases := map[string]struct {
		res               *T
		expected          []string
		hasOption         string
		hasOptionMatching string
		hasNotOptions     []string
	}{
		"use --net from run_args when no netns": {
			res: &T{
				BT: rescontainerocibase.BT{
					T: resource.T{ResourceID: &resourceid.T{
						Name: "id1",
					}},
					Path:     p,
					Hostname: "node1",
					RunArgs: []string{
						"--net", "container:netValue1",
					},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--hostname", "node1",
				"--net", "container:netValue1",
			},
			hasOption:         "--net",
			hasOptionMatching: "^container:netValue1$",
		},

		"set --net to netns value and remove extra --network when netns is defined": {
			res: &T{
				BT: rescontainerocibase.BT{
					T: resource.T{ResourceID: &resourceid.T{
						Name: "id1",
					}},
					Path:       p,
					Hostname:   "node1",
					Privileged: true,
					NetNS:      "host",
					RunArgs: []string{
						"-n", "fooX", "--detach", "--privileged",
						"--newOpt1", "newOpt1Value",
						"-h", "nodeX", "--hostname", "nodeY", "--net", "netValue1",
						"--network", "netValue2",
						"-v", "/etc/localtime:/etc/localtime:ro",
						"newOpt2",
					},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--hostname", "node1",
				"--privileged",
				"--net", "host",
				"--detach",
				"--newOpt1", "newOpt1Value",
				"-v", "/etc/localtime:/etc/localtime:ro",
				"newOpt2",
			},
			hasOption:         "--net",
			hasOptionMatching: "^host$",
			hasNotOptions:     []string{"--dns", "--dns-opt", "--dns-search"},
		},

		"use netns to set --net": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:     resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path:  p,
					NetNS: "netnsX",
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--net", "netnsX",
			},
			hasOption:         "--net",
			hasOptionMatching: "^netnsX$",
		},

		"no dns options when --network none": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:       resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path:    p,
					RunArgs: []string{"--network", "none"},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--network", "none",
			},
			hasOption:         "--network",
			hasOptionMatching: "^none$",
			hasNotOptions:     []string{"--dns", "--dns-opt", "--dns-search"},
		},

		"no dns options when --network container:...": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:       resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path:    p,
					RunArgs: []string{"--network", "container:..."},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--network", "container:...",
			},
			hasNotOptions: []string{"--dns", "--dns-opt", "--dns-search"},
		},

		"no dns options when --net container:...": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:       resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path:    p,
					RunArgs: []string{"--net", "container:..."},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--net", "container:...",
			},
			hasNotOptions: []string{"--dns", "--dns-opt", "--dns-search"},
		},

		"no dns options when --net names a network": {
			res: &T{
				BT: rescontainerocibase.BT{
					T: resource.T{ResourceID: &resourceid.T{
						Name: "id1",
					}},
					Path:     p,
					Hostname: "node1",
					RunArgs: []string{
						"--net", "netXX",
					},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--hostname", "node1",
				"--net", "netXX",
			},
			hasNotOptions: []string{"--dns", "--dns-opt", "--dns-search"},
		},

		"add --net none when no netns and no '--net' run_args, as docker": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:    resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path: p,
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--net", "none",
			},
			hasNotOptions: []string{"--dns", "--dns-opt", "--dns-search"},
		},

		// om writes the resolver of the container, so a resolver configuration
		// in run_args is a second one, in a place that no longer decides
		// anything. It is dropped rather than passed on.
		"keep a --network=x of run_args": {
			res: &T{
				BT: rescontainerocibase.BT{
					Path: p,
					T:    resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					RunArgs: []string{
						"--network=podman",
					},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--network=podman",
			},
		},
		"drop the resolver options of run_args": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:    resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path: p,
					RunArgs: []string{
						"--dns", "8.8.4.4",
						"--dns-opt", "ndots:5",
						"--dns-option", "edns0",
						"--dns-search", "other.example",
						"--memory", "1g",
					},
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--memory", "1g",
				"--net", "none",
			},
			hasNotOptions: []string{"--dns", "--dns-opt", "--dns-option", "--dns-search"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ea := tc.res.executorArg()

			tc.res.configure(ea)

			runArgsBase, err := ea.RunArgsBase(context.Background())
			if err != nil {
				require.NoError(t, err)
			}

			runArgsBase.DropOptionAndAnyValue("--label")
			runArgsBase.DropOptionAndAnyValue("-e")

			t.Logf("from resource %#v\n", tc.res)
			t.Logf("found runArgs: %v\n", runArgsBase)

			if tc.hasOption != "" {
				if tc.hasOptionMatching == "" {
					require.Truef(t, runArgsBase.HasOption(tc.hasOption), "wanted option %s\n got: %s", tc.hasOption, runArgsBase.Get())
				} else {
					require.Truef(t, runArgsBase.HasOptionAndMatchingValue(tc.hasOption, tc.hasOptionMatching), "wanted option %s matching %s\n got: %s",
						tc.hasOption, tc.hasOptionMatching, runArgsBase.Get())
				}
			}
			if len(tc.hasNotOptions) > 0 {
				for _, s := range tc.hasNotOptions {
					found := runArgsBase.HasOption(s) || runArgsBase.HasOptionAndAnyValue(s)
					require.Falsef(t, found, "unexpected option %s\n got: %s", s, runArgsBase.Get())
				}
			}

			if len(tc.expected) > 0 {
				t.Logf("expected runArgs: %v\n", tc.expected)
				require.ElementsMatchf(t, tc.expected, runArgsBase.Get(), "want: %s\ngot:  %s", tc.expected, runArgsBase.Get())
			}
		})
	}
}

// A container with its own netns may listen on any port as a user not
// root, as with docker: podman leaves the kernel default, 1024. A netns of
// the node or of another container is not the container's to set, and the
// setting of the user wins.
func TestUnprivilegedPortsSysctlWanted(t *testing.T) {
	prev := unprivilegedPortsSysctlFile
	unprivilegedPortsSysctlFile = t.TempDir()
	defer func() { unprivilegedPortsSysctlFile = prev }()

	for args, want := range map[string]bool{
		"--net none":            true,
		"--network podman":      true,
		"--network=podman":      true,
		"--net host":            false,
		"--net=container:abc":   false,
		"--net ns:/run/netns/x": false,
		"--net none --sysctl net.ipv4.ip_unprivileged_port_start=80": false,
		"--net none --sysctl=net.ipv4.ip_unprivileged_port_start=80": false,
		"--net none --sysctl net.ipv4.ip_forward=1":                  true,
	} {
		assert.Equal(t, want, unprivilegedPortsSysctlWanted(strings.Fields(args)), args)
	}

	unprivilegedPortsSysctlFile = "/nonexistent"
	assert.False(t, unprivilegedPortsSysctlWanted([]string{"--net", "none"}), "a kernel older than 4.11 has no such sysctl")
}
