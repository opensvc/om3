package rescontainerpodman

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

func TestExecutorArg_RunArgsBase(t *testing.T) {
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

		"has dns options when --net is not 'none' or 'container:...'": {
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
				"--dns-opt", "ndots:2", "--dns-opt", "edns0", "--dns-opt", "use-vc",
				"--net", "netXX",
			},
		},

		"don't add --net and defines dns options when no netns and no '--net' run_args": {
			res: &T{
				BT: rescontainerocibase.BT{
					T:    resource.T{ResourceID: &resourceid.T{Name: "id1"}},
					Path: p,
				},
			},
			expected: []string{
				"container", "run", "--name", "foo.id1",
				"--dns-opt", "ndots:2", "--dns-opt", "edns0", "--dns-opt", "use-vc",
			},
			hasNotOptions: []string{"--net"},
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
