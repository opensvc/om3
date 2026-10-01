package tui

import (
	"reflect"
	"testing"

	"github.com/opensvc/om3/v3/daemon/api"
)

// The flags of an action are the booleans of the api request it sends, a
// query parameter or a body member, named as the command line names them,
// less the ones the TUI does not offer.
func TestDoFlagsAreTheAPIBooleans(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  []string
		want []string
	}{
		{"instance provision", doInstance["provision"].flags(), []string{"--disable-rollback", "--force", "--leader", "--master", "--slaves", "--state-only"}},
		{"instance stop", doInstance["stop"].flags(), []string{"--force", "--interrupt-syncs", "--master", "--slaves"}},
		{"instance takeover", doInstance["takeover"].flags(), []string{"--interrupt-syncs", "--live"}},
		{"resource run", doResource["run"].flags(), []string{"--force", "--master", "--slaves"}},
		{"object restart", doObject["restart"].flags(), []string{"--force"}},
		{"object stop", doObject["stop"].flags(), []string{"--interrupt-syncs"}},
		{"object start", doObject["start"].flags(), nil},
	} {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// The flags typed are set on the request, a pointer boolean as a plain one,
// and the other arguments are the words of the action.
func TestParseDoArgs(t *testing.T) {
	params := &api.PostInstanceActionProvisionParams{}
	words, err := parseDoArgs(params, []string{"--state-only", "--leader"})
	if err != nil || len(words) != 0 {
		t.Fatalf("words %v, err %v", words, err)
	}
	if params.StateOnly == nil || !*params.StateOnly || params.Leader == nil || !*params.Leader {
		t.Errorf("flags not set: %+v", params)
	}
	if params.Force != nil || params.DisableRollback != nil {
		t.Errorf("flags set that were not typed: %+v", params)
	}

	body := &api.PostObjectActionSwitch{}
	if _, err := parseDoArgs(body, []string{"--live", "--interrupt-syncs"}); err != nil {
		t.Fatal(err)
	}
	if !body.Live || body.InterruptSyncs == nil || !*body.InterruptSyncs {
		t.Errorf("body flags not set: %+v", body)
	}

	words, err = parseDoArgs(nil, []string{"10g"})
	if err != nil || len(words) != 1 || words[0] != "10g" {
		t.Errorf("words %v, err %v", words, err)
	}
}

// A flag the request does not have is refused, whatever the action: a flag
// of another action, a hidden one, a value parameter, and any flag of an
// action taking none.
func TestParseDoArgsRefusesUnknownFlags(t *testing.T) {
	for _, tc := range []struct {
		req  any
		flag string
	}{
		{&api.PostInstanceActionStopParams{}, "--state-only"},
		{&api.PostInstanceActionRunParams{}, "--cron"},
		{&api.PostInstanceActionRunParams{}, "--confirm"},
		{&api.PostInstanceActionStopParams{}, "--rid"},
		{&api.PostInstanceActionStopParams{}, "--watch"},
		{nil, "--force"},
	} {
		if _, err := parseDoArgs(tc.req, []string{tc.flag}); err == nil {
			t.Errorf("%T accepted %s", tc.req, tc.flag)
		}
	}
}

// The completion offers the actions that run, no more and no less, with
// their flags and their words.
func TestCompletionOffersWhatRuns(t *testing.T) {
	check := func(scope string, tree node, actions []string, flags map[string][]string) {
		if len(tree) != len(actions) {
			t.Errorf("%s: %d actions offered, %d run", scope, len(tree), len(actions))
		}
		for _, action := range actions {
			sub, ok := tree[action]
			if !ok {
				t.Errorf("%s %s runs and is not offered", scope, action)
				continue
			}
			for _, flag := range flags[action] {
				if _, ok := sub[flag]; !ok {
					t.Errorf("%s %s takes %s and does not offer it", scope, action, flag)
				}
			}
		}
	}
	names := func(m any) []string {
		var l []string
		for _, k := range reflect.ValueOf(m).MapKeys() {
			l = append(l, k.String())
		}
		return l
	}
	flags := DoFlags()
	check("cluster", doCluster.node(), names(doCluster), flags["cluster"])
	check("node", doNode.node(), names(doNode), flags["node"])
	check("object", doObject.node(), names(doObject), flags["object"])
	check("instance", doInstance.node(), names(doInstance), flags["instance"])
	check("resource", doResource.node(), names(doResource), flags["resource"])
	if _, ok := doNode.node()["daemon"]["restart"]; !ok {
		t.Error("node daemon does not offer restart")
	}
}

// A flag hidden is a boolean some request has: a name no request has hides
// nothing, and is a parameter renamed or removed since.
func TestHiddenFlagsExist(t *testing.T) {
	seen := make(map[string]bool)
	note := func(req any) {
		v := reflect.TypeOf(req).Elem()
		for i := 0; i < v.NumField(); i++ {
			for _, tag := range []string{"form", "json"} {
				name := v.Field(i).Tag.Get(tag)
				for j, c := range name {
					if c == ',' {
						name = name[:j]
						break
					}
				}
				seen[name] = true
			}
		}
	}
	for _, a := range doResource {
		if a.request != nil {
			note(a.request())
		}
	}
	for _, a := range doInstance {
		if a.request != nil {
			note(a.request())
		}
	}
	for _, a := range doObject {
		if a.request != nil {
			note(a.request())
		}
	}
	for name := range doHiddenFlags {
		if !seen[name] {
			t.Errorf("hidden flag %s is a parameter of no request", name)
		}
	}
}

// The command line runs an action with the flags typed set on its request,
// under its alias too, and runs nothing when a flag, a word or the action
// itself is not one it knows.
func TestRunDo(t *testing.T) {
	var (
		ran    int
		params *api.PostInstanceActionStopParams
	)
	actions := doActions[selInstances]{
		"stop": flagged(func(_ *App, _ selInstances, p *api.PostInstanceActionStopParams) {
			ran++
			params = p
		}),
		"unfreeze": plain(func(*App, selInstances) { ran++ }),
	}
	app := &App{errsC: make(chan errsMessage, 8)}
	refused := func() bool {
		select {
		case <-app.errsC:
			return true
		default:
			return false
		}
	}

	runDo(app, "instance", actions, []string{"stop", "--force"}, nil)
	if ran != 1 || params == nil || params.Force == nil || !*params.Force || refused() {
		t.Fatalf("stop --force: ran %d, params %+v", ran, params)
	}
	runDo(app, "instance", actions, []string{"thaw"}, nil)
	if ran != 2 || refused() {
		t.Errorf("thaw: ran %d", ran)
	}
	for _, args := range [][]string{
		{"stop", "--bogus"},
		{"stop", "--force", "--state-only"},
		{"unfreeze", "--force"},
		{"unfreeze", "extra"},
		{"delete"},
	} {
		runDo(app, "instance", actions, args, nil)
		if ran != 2 || !refused() {
			t.Errorf("%v: ran %d, not refused", args, ran)
		}
	}
}
