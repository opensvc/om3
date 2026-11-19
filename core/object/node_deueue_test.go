package object

import (
	"reflect"
	"testing"
)

func TestQueuedActionCmdArgs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action QueuedAction
		want   []string
	}{
		{"node", QueuedAction{Command: "pushasset"}, []string{"node", "pushasset"}},
		{"node selector", QueuedAction{Command: "freeze --node *"}, []string{"node", "freeze", "--node", "*"}},
		{"cluster", QueuedAction{Command: "cluster update pkg"}, []string{"cluster", "update", "pkg"}},
		{"service", QueuedAction{Svcname: "svc1", Command: "start"}, []string{"svc", "-s", "svc1", "start"}},
		{"service named cluster", QueuedAction{Svcname: "svc1", Command: "cluster"}, []string{"svc", "-s", "svc1", "cluster"}},
	} {
		got, err := tc.action.cmdArgs()
		if err != nil {
			t.Fatalf("%s: %s", tc.name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
