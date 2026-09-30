package tui

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// doAction is one action of the ":do" command, on a selection of kind S.
	//
	// It is declared once, in the table of its kind of selection, and both
	// the completion and the command line are read from that table: an
	// action offered is an action run, and a flag offered is a flag the api
	// takes.
	doAction[S any] struct {
		// request allocates the api request the action sends. Its booleans
		// are the flags of the action, so a flag the api gains is offered
		// and accepted with no list to keep up. It is nil for an action
		// taking no flag.
		request func() any

		// words are the words the action takes after its name, as the size
		// of a resize, and the ones the completion offers among them.
		takesWords bool
		words      node

		// confirm returns what the user confirms before the action runs.
		// It is nil for an action run unconfirmed.
		confirm func(req any) []string

		run func(t *App, sel S, req any, words []string)
	}

	doActions[S any] map[string]doAction[S]

	// The kinds of selection an action applies to: nothing for the
	// cluster, node names, object paths, {path, node} and
	// {path, node, rid}.
	selCluster   = struct{}
	selNodes     = map[string]any
	selObjects   = map[string]any
	selInstances = map[[2]string]any
	selResources = map[[3]string]any
)

var (
	// doAliases are the action names accepted and not offered.
	doAliases = map[string]string{
		"thaw": "unfreeze",
	}

	// doHiddenFlags are the booleans of the api requests the TUI does not
	// offer, by the name of their parameter.
	doHiddenFlags = map[string]string{
		"cron":    "says the daemon scheduler runs the action, which a user is not",
		"confirm": "a run from the TUI is confirmed, the TUI having no prompt to answer",
	}

	doCluster  doActions[selCluster]
	doNode     doActions[selNodes]
	doObject   doActions[selObjects]
	doInstance doActions[selInstances]
	doResource doActions[selResources]
)

// The tables are filled at init rather than in their declaration: an action
// reaching back to the command line that runs it would be an initialization
// cycle.
func init() {
	doCluster = doActions[selCluster]{
		"freeze":   unselected((*App).actionClusterFreeze),
		"unfreeze": unselected((*App).actionClusterUnfreeze),
	}
	doNode = doActions[selNodes]{
		"daemon":   worded((*App).actionNodeDaemon, node{"restart": nil}),
		"drain":    plain((*App).actionNodeDrain),
		"freeze":   plain((*App).actionNodeFreeze),
		"unfreeze": plain((*App).actionNodeUnfreeze),
	}
	doObject = doActions[selObjects]{
		"abort":       plain((*App).actionAbort),
		"delete":      plain((*App).actionDelete).confirmed(confLostMessage),
		"freeze":      plain((*App).actionFreeze),
		"giveback":    plain((*App).actionGiveback).confirmed(serviceInterruptionMessage),
		"provision":   plain((*App).actionProvision),
		"purge":       plain((*App).actionPurge).confirmed(dataLostMessage, confLostMessage, serviceInterruptionMessage),
		"resize":      worded((*App).actionResize, nil),
		"restart":     flagged((*App).actionRestart).confirmed(serviceInterruptionMessage),
		"start":       plain((*App).actionStart),
		"stop":        flagged((*App).actionStop).confirmed(serviceInterruptionMessage),
		"switch":      flagged((*App).actionSwitch).confirmed(serviceInterruptionMessage),
		"unfreeze":    plain((*App).actionUnfreeze),
		"unprovision": plain((*App).actionUnprovision).confirmed(dataLostMessage, serviceInterruptionMessage),
	}
	doInstance = doActions[selInstances]{
		"clear":       plain((*App).actionInstanceClear),
		"freeze":      flagged((*App).actionInstanceFreeze),
		"provision":   flagged((*App).actionInstanceProvision),
		"refresh":     plain((*App).actionInstanceRefresh),
		"restart":     flagged((*App).actionInstanceRestart).confirmed(serviceInterruptionMessage),
		"start":       flagged((*App).actionInstanceStart),
		"stop":        flagged((*App).actionInstanceStop).confirmed(serviceInterruptionMessage),
		"switch":      flagged((*App).actionInstanceSwitch).confirmed(serviceInterruptionMessage),
		"takeover":    flagged((*App).actionInstanceSwitch).confirmed(serviceInterruptionMessage),
		"unfreeze":    flagged((*App).actionInstanceUnfreeze),
		"unprovision": flagged((*App).actionInstanceUnprovision).confirmedBy(unprovisionConfirmation),
	}
	doResource = doActions[selResources]{
		"disable":     plain((*App).actionResourceDisable).confirmed(serviceInterruptionMessage),
		"enable":      plain((*App).actionResourceEnable),
		"provision":   flagged((*App).actionResourceProvision),
		"restart":     flagged((*App).actionResourceRestart).confirmed(serviceInterruptionMessage),
		"run":         flagged((*App).actionResourceRun),
		"start":       flagged((*App).actionResourceStart),
		"stop":        flagged((*App).actionResourceStop).confirmed(serviceInterruptionMessage),
		"unprovision": flagged((*App).actionResourceUnprovision).confirmedBy(unprovisionConfirmation),
	}
}

// plain declares an action taking no flag and no word.
func plain[S any](run func(*App, S)) doAction[S] {
	return doAction[S]{
		run: func(t *App, sel S, _ any, _ []string) { run(t, sel) },
	}
}

// unselected declares an action of the cluster, which has no selection.
func unselected(run func(*App)) doAction[selCluster] {
	return doAction[selCluster]{
		run: func(t *App, _ selCluster, _ any, _ []string) { run(t) },
	}
}

// flagged declares an action whose flags are the booleans of the api request
// R it sends.
func flagged[S, R any](run func(*App, S, *R)) doAction[S] {
	return doAction[S]{
		request: func() any { return new(R) },
		run:     func(t *App, sel S, req any, _ []string) { run(t, sel, req.(*R)) },
	}
}

// worded declares an action taking words after its name, and the ones the
// completion offers.
func worded[S any](run func(*App, S, []string), words node) doAction[S] {
	return doAction[S]{
		takesWords: true,
		words:      words,
		run:        func(t *App, sel S, _ any, words []string) { run(t, sel, words) },
	}
}

// confirmed makes the user confirm the messages before the action runs.
func (a doAction[S]) confirmed(messages ...string) doAction[S] {
	a.confirm = func(any) []string { return messages }
	return a
}

// confirmedBy makes the user confirm what the request, with its flags set,
// is about to do.
func (a doAction[S]) confirmedBy(confirm func(req any) []string) doAction[S] {
	a.confirm = confirm
	return a
}

// flags are the flags the action takes, sorted.
func (a doAction[S]) flags() []string {
	if a.request == nil {
		return nil
	}
	fields := doFlagFields(a.request())
	l := make([]string, 0, len(fields))
	for flag := range fields {
		l = append(l, flag)
	}
	sort.Strings(l)
	return l
}

// node is the completion tree of the actions: each action, with the flags
// and the words it takes.
func (m doActions[S]) node() node {
	n := make(node)
	for name, a := range m {
		var sub node
		add := func(word string, next node) {
			if sub == nil {
				sub = make(node)
			}
			sub[word] = next
		}
		for _, flag := range a.flags() {
			add(flag, nil)
		}
		for word, next := range a.words {
			add(word, next)
		}
		n[name] = sub
	}
	return n
}

// flagsByAction are the flags of each action taking some.
func (m doActions[S]) flagsByAction() map[string][]string {
	out := make(map[string][]string)
	for name, a := range m {
		if l := a.flags(); len(l) > 0 {
			out[name] = l
		}
	}
	return out
}

// DoFlags returns the flags the ":do" command offers, by kind of selection
// then by action, for the command tree of ox to be checked against: a user
// typing a flag in the TUI types the one the command line takes.
func DoFlags() map[string]map[string][]string {
	return map[string]map[string][]string{
		"cluster":  doCluster.flagsByAction(),
		"node":     doNode.flagsByAction(),
		"object":   doObject.flagsByAction(),
		"instance": doInstance.flagsByAction(),
		"resource": doResource.flagsByAction(),
	}
}

// doFlagFields returns the boolean fields of an api request by the flag that
// sets them: a query parameter or a body member state_only is --state-only.
func doFlagFields(req any) map[string]reflect.Value {
	fields := make(map[string]reflect.Value)
	v := reflect.ValueOf(req)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fields
	}
	v = v.Elem()
	for i := 0; i < v.NumField(); i++ {
		ft := v.Type().Field(i)
		kind := ft.Type
		if kind.Kind() == reflect.Ptr {
			kind = kind.Elem()
		}
		if kind.Kind() != reflect.Bool {
			continue
		}
		tag := ft.Tag.Get("form")
		if tag == "" {
			tag = ft.Tag.Get("json")
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		if _, ok := doHiddenFlags[name]; ok {
			continue
		}
		fields["--"+strings.ReplaceAll(name, "_", "-")] = v.Field(i)
	}
	return fields
}

// parseDoArgs sets the flags named in the arguments on the api request, and
// returns the other arguments, the words of the action.
func parseDoArgs(req any, args []string) ([]string, error) {
	var (
		fields map[string]reflect.Value
		words  []string
	)
	if req != nil {
		fields = doFlagFields(req)
	}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			words = append(words, arg)
			continue
		}
		field, ok := fields[arg]
		if !ok {
			return nil, fmt.Errorf("unsupported option: %s", arg)
		}
		if field.Kind() == reflect.Ptr {
			p := reflect.New(field.Type().Elem())
			p.Elem().SetBool(true)
			field.Set(p)
		} else {
			field.SetBool(true)
		}
	}
	return words, nil
}

// runDo runs an action of the ":do" command on a selection: the flags typed
// are set on the api request the action sends, and the action is confirmed
// when it has something to confirm.
func runDo[S any](t *App, scope string, actions doActions[S], args []string, sel S) {
	name := args[0]
	if alias, ok := doAliases[name]; ok {
		name = alias
	}
	a, ok := actions[name]
	if !ok {
		t.errorf("unknown %s action: %s", scope, args[0])
		return
	}
	var req any
	if a.request != nil {
		req = a.request()
	}
	words, err := parseDoArgs(req, args[1:])
	if err != nil {
		t.errorf("%s", err)
		return
	}
	if len(words) > 0 && !a.takesWords {
		t.errorf("unexpected argument: %s", words[0])
		return
	}
	run := func() { a.run(t, sel, req, words) }
	if a.confirm != nil {
		if messages := a.confirm(req); len(messages) > 0 {
			t.confirmAction(run, messages...)
			return
		}
	}
	run()
}

// unprovisionConfirmation is what the user confirms an unprovision with.
func unprovisionConfirmation(req any) []string {
	params, ok := req.(*api.PostInstanceActionUnprovisionParams)
	return unprovisionMessages(ok && params.StateOnly != nil && *params.StateOnly)
}

// unprovisionMessages are what the user confirms an unprovision with: a
// state only one destroys nothing and stops nothing, it only marks the
// resources unprovisioned.
func unprovisionMessages(stateOnly bool) []string {
	if stateOnly {
		return []string{stateOnlyUnprovisionMessage}
	}
	return []string{dataLostMessage, serviceInterruptionMessage}
}
