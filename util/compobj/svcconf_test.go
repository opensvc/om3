package main

import (
	"fmt"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/stretchr/testify/require"
	"os"
	"os/user"
	"testing"
)

func TestSvcconfAdd(t *testing.T) {
	usr, err := user.Current()
	require.NoError(t, err)
	if usr.Username != "root" {
		t.Skip("need root")
	}

	require.NoError(t, os.Setenv("OSVC_COMP_SERVICES_SVCNAME", "svcTest"))
	defer func() { require.NoError(t, os.Unsetenv("OSVC_COMP_SERVICES_SVCNAME")) }()

	p, err := naming.ParsePath("svcTest")
	require.NoError(t, err)
	s, err := object.NewSvc(p)
	require.NoError(t, err)
	require.NoError(t, s.Config().SetKeys(keyop.ParseList("app#0.start=test", "app#1.start=test1")...))
	fmt.Println(s.Config().SectionStrings())
	fmt.Println(p)
	obj := CompSvcconfs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	require.Error(t, obj.Add(`[{}]`))
	require.Equal(t, []string{"DEFAULT", "app#0", "app#1"}, svcRessourcesNames)

	testCases := map[string]struct {
		jsonRule      string
		expectError   bool
		expectedRules []any
	}{
		"add with a true simple rule": {
			jsonRule:    `[{"key" : "test", "op" : "=", "value" : 5}]`,
			expectError: false,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: "5",
			}},
		},

		"add a rule with no key": {
			jsonRule:    `[{"op" : "=", "value" : 5}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},

		"add a rule with no value": {
			jsonRule:    `[{"key" : "test", "op" : "="}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},

		"add multiple rules": {
			jsonRule:    `[{"key" : "test", "op" : "=", "value" : 5}, {"key" : "test2", "op" : ">=", "value" : 3}]`,
			expectError: false,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: "5",
			}, CompNodeconf{
				Key:   "test2",
				Op:    ">=",
				Value: "3",
			}},
		},

		"with an operator that is not in =, <=, >=": {
			jsonRule:    `[{"key" : "test", "op" : ">>", "value" : 5}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompSvcconfs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			err := obj.Add(c.jsonRule)
			if c.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expectedRules, obj.rules)
			}
		})
	}
}

func TestSvcconfCheckRule(t *testing.T) {
	usr, err := user.Current()
	require.NoError(t, err)
	if usr.Username != "root" {
		t.Skip("need root")
	}

	oriSvcName := svcName
	defer func() { svcName = oriSvcName }()
	svcName = "svcTest"

	s, err := object.NewConfigurer(svcName)
	require.NoError(t, err)

	testCases := map[string]struct {
		rule                CompSvcconf
		expectedCheckResult ExitCode
	}{
		"with a true rule and no filter": {
			rule: CompSvcconf{
				Key:   "app.start",
				Op:    "=",
				Value: "test",
			},
			expectedCheckResult: ExitOk,
		},

		"with a false rule and no filter": {
			rule: CompSvcconf{
				Key:   "app.start",
				Op:    "=",
				Value: "false",
			},
			expectedCheckResult: ExitNok,
		},

		"with a true rule and no filter the full section name is precised": {
			rule: CompSvcconf{
				Key:   "container#0.type",
				Op:    "=",
				Value: "vbox",
			},
			expectedCheckResult: ExitOk,
		},

		"with a true rule and a filter": {
			rule: CompSvcconf{
				Key:   "container(name=v).type",
				Op:    "=",
				Value: "vbox",
			},
			expectedCheckResult: ExitOk,
		},

		"with a false rule and a filter": {
			rule: CompSvcconf{
				Key:   "container(name=d).type",
				Op:    "=",
				Value: "vbox",
			},
			expectedCheckResult: ExitNok,
		},

		"with a filter that correspond to nothing": {
			rule: CompSvcconf{
				Key:   "container(name=lalaal).type",
				Op:    "=",
				Value: "vboxlala",
			},
			expectedCheckResult: ExitOk,
		},

		"with a filter with && and a false rule": {
			rule: CompSvcconf{
				Key:   "container(name=v&&stop_timeout=8).type",
				Op:    "=",
				Value: "vboxlala",
			},
			expectedCheckResult: ExitNok,
		},

		"with a filter with && and a true rule": {
			rule: CompSvcconf{
				Key:   "container(name=v&&stop_timeout=8).type",
				Op:    "=",
				Value: "vbox",
			},
			expectedCheckResult: ExitOk,
		},

		"with a filter with || and a true rule": {
			rule: CompSvcconf{
				Key:   "container(name=v||name=d).same",
				Op:    "=",
				Value: "a",
			},
			expectedCheckResult: ExitOk,
		},

		"with a filter with || and a false rule": {
			rule: CompSvcconf{
				Key:   "container(name=v||name=d).same",
				Op:    "=",
				Value: "alal",
			},
			expectedCheckResult: ExitNok,
		},

		"with a filter with one || and then one && (in the filter the part on the left of || is true) and a true rule": {
			rule: CompSvcconf{
				Key:   "container(same=a||name=vbox&&stop_timeout=8).same",
				Op:    "=",
				Value: "a",
			},
			expectedCheckResult: ExitOk,
		},

		"with a filter with one || and then one && (in the filter the part on the right of || is true) and a true rule": {
			rule: CompSvcconf{
				Key:   "container(same=vv||name=v&&stop_timeout=8).name",
				Op:    "=",
				Value: "v",
			},
			expectedCheckResult: ExitOk,
		},

		"with a filter with one || and then one && (in the filter the part on the right of || is true) and a false rule": {
			rule: CompSvcconf{
				Key:   "container(same=vv||name=v&&stop_timeout=8).name",
				Op:    "=",
				Value: "vboxlala",
			},
			expectedCheckResult: ExitNok,
		},

		"with wrong key": {
			rule: CompSvcconf{
				Key:   "container#1",
				Op:    "=",
				Value: "vboxlala",
			},
			expectedCheckResult: ExitOk,
		},

		"with a rule with <=": {
			rule: CompSvcconf{
				Key:   "container#0.stop_timeout",
				Op:    "<=",
				Value: "10",
			},
			expectedCheckResult: ExitOk,
		},
	}

	obj := CompSvcconfs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, s.Config().SetKeys(keyop.ParseList("app#0.start=test", "app#1.start=test", "container#0.type=vbox", "container#0.name=v", "container#0.stop_timeout=8", "container#0.same=a", "container#1.type=docker", "container#1.name=d", "container#1.stop_timeout=8", "container#1.same=a")...))
			svcRessourcesNames = s.Config().SectionStrings()
			require.Equal(t, c.expectedCheckResult, obj.checkRule(c.rule))
			require.Equal(t, ExitOk, obj.fixRule(c.rule))
			require.Equal(t, ExitOk, obj.checkRule(c.rule))
			require.NoError(t, s.Config().DeleteSections([]string{"app#0", "app#1", "container#0", "container#1"}))
			fmt.Println(s.Config().SectionStrings())
		})
	}
}
