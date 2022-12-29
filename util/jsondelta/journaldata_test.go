package jsondelta

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	MonitorThreadStatus struct {
		Nodes map[string]NodeStatus `json:"nodes"`
	}
	NodeStatus struct {
		Agent    string       `json:"agent"`
		Services NodeServices `json:"services"`
	}
	Config struct {
		Checksum string   `json:"csum"`
		Scope    []string `json:"scope"`
	}
	NodeServices struct {
		Config map[string]Config `json:"config"`
	}
)

func getCfg(c *JournalData, node, name string) *Config {
	b, err := c.MarshalPath(OperationPath{"cluster", "node", node, "services", "config", name})
	if err != nil {
		return nil
	}
	cfg := &Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		return nil
	}
	return cfg
}

type journal struct {
	patch Patch
}

func (n *journal) reset() {
	n.patch = make(Patch, 0)
}

func (n *journal) patchEvent(ops Patch) {
	n.patch = append(n.patch, ops...)
}

func (n *journal) show(t *testing.T) {
	for id, op := range n.patch {
		t.Logf("patch %d: %s", id, op.Render())
	}
}

func getInitialConfig(csum string) *Config {
	return &Config{
		Checksum: csum,
		Scope:    []string{"scope0", "scope1"},
	}
}

func getInitialMonitor(s []string) *MonitorThreadStatus {
	configs := make(map[string]Config)
	for _, s := range s {
		configs[s] = *getInitialConfig("initial csum " + s)
	}
	return &MonitorThreadStatus{
		Nodes: map[string]NodeStatus{
			"node1": {
				Agent:    "v3.0-dev",
				Services: NodeServices{Config: configs},
			},
		},
	}
}

func TestJournal(t *testing.T) {
	// TODO remove this file
	t.Skip("not anymore used")
	t.Run("checkVsPseudoClusterData", func(t *testing.T) {
		// TODO remove when moved to daemondata
		tJournal := &journal{patch: make([]Operation, 0)}
		container := New(tJournal)
		var err error
		svc1 := "Svc1"
		monitor := getInitialMonitor([]string{svc1})
		svcToAdd := []string{"Svc2", "Svc3"}
		fromSvcConfigPath := func(svc string, v ...interface{}) OperationPath {
			return append(OperationPath{"cluster", "node", "node1",
				"services", "config", svc}, v...)
		}

		t.Run("set initial monitor", func(t *testing.T) {
			path := OperationPath{"monitor"}
			tJournal.reset()
			err = container.Set(path, monitor)
			require.Nil(t, err)
			require.Equal(t, "initial csum Svc1", getCfg(container, "node1", svc1).Checksum)
			tJournal.show(t)
			expectedOps := Patch{
				{
					OpPath:  path,
					OpValue: NewOptValue(monitor),
					OpKind:  "replace",
				},
			}
			assert.Equal(t, expectedOps, tJournal.patch)
		})

		t.Run("when sub key change", func(t *testing.T) {
			tJournal.reset()
			expectedCsum := "new checksum"

			err := container.Set(fromSvcConfigPath(svc1), getInitialConfig(expectedCsum))

			tJournal.show(t)
			require.Nil(t, err)
			expectedOps := Patch{
				{
					OpPath:  append(fromSvcConfigPath(svc1, "csum")),
					OpValue: NewOptValue(expectedCsum),
					OpKind:  "replace",
				},
			}
			require.Equal(t, expectedOps, tJournal.patch)
			require.Equal(t, expectedCsum, getCfg(container, "node1", svc1).Checksum)
		})

		t.Run("when it is a new path", func(t *testing.T) {
			for _, name := range svcToAdd {
				path := fromSvcConfigPath(name)
				t.Run(path.String(), func(t *testing.T) {
					tJournal.reset()

					err = container.Set(path, getInitialConfig("Foo"))

					tJournal.show(t)
					require.Nil(t, err)
					expectedPatch := Patch{
						{
							OpPath:  fromSvcConfigPath(name),
							OpValue: NewOptValue(getInitialConfig("Foo")),
							OpKind:  "replace",
						},
					}
					require.Equal(t, expectedPatch, tJournal.patch)
				})
			}

			t.Run("Keys", func(t *testing.T) {
				path := OperationPath{"cluster", "node", "node1", "services", "config"}
				keys, err := container.Keys(path)
				t.Logf("found keys: %s", keys)
				require.Equal(t, []string{"Svc1", "Svc2", "Svc3"}, keys)
				require.Nil(t, err)
			})
		})

		t.Run("set initial monitor on updated monitor", func(t *testing.T) {
			t.Run("set set initial monitor", func(t *testing.T) {
				tJournal.reset()

				err = container.Set(OperationPath{"monitor"}, monitor)
				tJournal.show(t)
				require.NoError(t, err)

				var expectedOps Patch
				for _, svcname := range svcToAdd {
					expectedOps = append(expectedOps, Operation{
						OpPath: fromSvcConfigPath(svcname),
						OpKind: "remove",
					})
				}
				expectedOps = append(expectedOps, Operation{
					OpPath:  fromSvcConfigPath(svc1, "csum"),
					OpValue: NewOptValue(getInitialConfig("initial csum " + svc1).Checksum),
					OpKind:  "replace",
				})

				require.ElementsMatch(t, expectedOps, tJournal.patch)
			})

			t.Run("replace monitor with other initial config", func(t *testing.T) {
				for _, name := range []string{"Svc1", "Svc2", "Svc3"} {
					path := fromSvcConfigPath(name)
					t.Run("add "+path.String(), func(t *testing.T) {
						tJournal.reset()
						expectedCheckSum := "new csum for " + name

						err = container.Set(path, getInitialConfig(expectedCheckSum))

						tJournal.show(t)
						require.Nil(t, err)
						require.Equal(t, expectedCheckSum, getCfg(container, "node1", name).Checksum)
					})
				}
			})

			t.Run("replace array item", func(t *testing.T) {
				tJournal.reset()
				newScope0 := "SCOPE0"

				err = container.Set(fromSvcConfigPath(svc1, "scope", 0), newScope0)

				tJournal.show(t)
				require.Nil(t, err)

				expectedOps := Patch{
					{
						OpPath:  fromSvcConfigPath(svc1, "scope", 0),
						OpValue: NewOptValue(newScope0),
						OpKind:  "replace",
					},
				}
				require.Equal(t, expectedOps, tJournal.patch)
				require.Equal(t, []string{newScope0, "scope1"}, getCfg(container, "node1", svc1).Scope)
			})

			t.Run("add other configs", func(t *testing.T) {
				for _, name := range []string{"Svc1", "Svc2", "Svc3", "iSvc"} {
					tJournal.reset()
					path := fromSvcConfigPath(name)
					t.Run("add "+path.String(), func(t *testing.T) {
						err = container.Set(path, getInitialConfig("other config for "+name))
						require.Nil(t, err)
						tJournal.show(t)
					})
				}
				t.Run("replace monitor with alternate service configs", func(t *testing.T) {
					tJournal.reset()
					path := OperationPath{"monitor"}
					err = container.Set(path, getInitialMonitor([]string{"alt1", "alt2"}))
					require.NoError(t, err)
					tJournal.show(t)
				})
			})
		})
		t.Run("when missing path", func(t *testing.T) {
			tJournal.reset()
			err = container.Set(OperationPath{"foo", "bar"}, monitor)
			require.NotNil(t, err)
			require.Len(t, tJournal.patch, 0)
		})
	})

	t.Run("b2.1", func(t *testing.T) {
		tJournal := &journal{patch: make(Patch, 0)}
		container := New(tJournal)
		type testCase struct {
			kind  string
			path  OperationPath
			value interface{}
			patch Patch
		}
		cases := []testCase{
			{
				kind: "replace",
				path: OperationPath{"a"},
				value: map[string]interface{}{
					"b":          0,
					"c":          []int{1, 2},
					"aBoolTrue":  true,
					"aBoolFalse": false,
					"aTrue":      true,
					"aFalse":     false,
					"d":          map[string]string{"da": ""},
				},
				patch: Patch{
					{
						OpPath: OperationPath{"a"},
						OpValue: NewOptValue(map[string]interface{}{
							"aBoolTrue":  true,
							"aBoolFalse": false,
							"aFalse":     false,
							"aTrue":      true,
							"b":          0,
							"c":          []int{1, 2},
							"d":          map[string]string{"da": ""},
						}),
						OpKind: "replace",
					},
				},
			},
			{
				kind: "replace",
				path: OperationPath{"a"},
				value: map[string]interface{}{
					"b":          1,
					"aBoolTrue":  false,
					"aBoolFalse": true,
					"c":          []int{1, 2, 3},
					"e":          map[string]int{"ea": 1, "eb": 2},
					"E":          map[string]int{"ea": 1, "eb": 2},
				},
				patch: Patch{
					{
						OpPath: OperationPath{"a", "d"},
						OpKind: "remove",
					},
					{
						OpPath: OperationPath{"a", "aFalse"},
						OpKind: "remove",
					},
					{
						OpPath: OperationPath{"a", "aTrue"},
						OpKind: "remove",
					},
					{
						OpPath:  OperationPath{"a", "aBoolFalse"},
						OpValue: NewOptValue(true),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "aBoolTrue"},
						OpValue: NewOptValue(false),
						OpKind:  "replace",
					},

					{
						OpPath:  OperationPath{"a", "b"},
						OpValue: NewOptValue(1),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "c", 2},
						OpValue: NewOptValue(3),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "e"},
						OpValue: NewOptValue(map[string]int{"ea": 1, "eb": 2}),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "E"},
						OpValue: NewOptValue(map[string]int{"ea": 1, "eb": 2}),
						OpKind:  "replace",
					},
				},
			},
			{
				kind: "remove",
				path: OperationPath{"a", "e"},
				patch: Patch{
					{
						OpPath: OperationPath{"a", "e"},
						OpKind: "remove",
					},
				},
			},
			{
				kind: "replace",
				path: OperationPath{"a"},
				value: map[string]interface{}{
					"bNew":          0,
					"c":             []int{8, 2},
					"aBoolTrueNew":  true,
					"aBoolFalseNew": false,
					"aTrueNew":      true,
					"aFalseNew":     false,
					"E":             map[string]int{"ea": 10, "eb": 20},
					"dNew":          map[string]string{"da": "DA", "db": "DB"},
				},
				patch: Patch{
					{
						OpPath: OperationPath{"a", "aBoolFalse"},
						OpKind: "remove",
					},
					{
						OpPath: OperationPath{"a", "aBoolTrue"},
						OpKind: "remove",
					},
					{
						OpPath: OperationPath{"a", "b"},
						OpKind: "remove",
					},
					{
						OpPath: OperationPath{"a", "c", 2},
						OpKind: "remove",
					},
					{
						OpPath:  OperationPath{"a", "aBoolFalseNew"},
						OpValue: NewOptValue(false),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "aBoolTrueNew"},
						OpValue: NewOptValue(true),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "aTrueNew"},
						OpValue: NewOptValue(true),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "aFalseNew"},
						OpValue: NewOptValue(false),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "bNew"},
						OpValue: NewOptValue(0),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "c", 0},
						OpValue: NewOptValue(8),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "dNew"},
						OpValue: NewOptValue(map[string]string{"da": "DA", "db": "DB"}),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "E", "ea"},
						OpValue: NewOptValue(10),
						OpKind:  "replace",
					},
					{
						OpPath:  OperationPath{"a", "E", "eb"},
						OpValue: NewOptValue(20),
						OpKind:  "replace",
					},
				},
			},
		}

		for id, o := range cases {
			t.Run(o.kind+" "+strconv.Itoa(id), func(t *testing.T) {
				var err error
				tJournal.reset()

				switch o.kind {
				case "replace":
					t.Logf("id: %d %s [%v] %v", id, o.kind, o.path.String(), o.value)
					err = container.Set(o.path, o.value)
				case "remove":
					t.Logf("id: %d %s [%v]", id, o.kind, o.path.String())
					err = container.Unset(o.path)
				}

				tJournal.show(t)

				require.ElementsMatch(t, tJournal.patch, o.patch)
				require.Nil(t, err)
			})
		}
	})
}
