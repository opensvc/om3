package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newRun(name string) *Bus {
	bus := NewBus(name)
	bus.Start(context.Background())
	return bus
}

func TestPub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	bus.Pub("foo", Label{"op", "create"})
	bus.Pub("foo", Label{"op", "update"})
	bus.Pub("foo", Label{"op", "read"})
	bus.Pub("foo", Label{"op", "delete"})
	bus.Pub("bar")
	bus.Pub("foobar")
}

func TestSub(t *testing.T) {
	type (
		testPub struct {
			v      interface{}
			labels []Label
		}
		testFilter struct {
			filterType interface{}
			labels     []Label
		}
	)
	cases := map[string]struct {
		filters  []testFilter
		pubs     []testPub
		expected []interface{}
	}{
		"publish with or without label, subscribe without label must receive all publications": {
			pubs: []testPub{
				{v: "foo"},
				{v: "pub with label", labels: []Label{{"xx", "XXX"}}},
				{v: "foo2"},
				{v: 1},
			},
			expected: []interface{}{"foo", "pub with label", "foo2", 1},
		},

		"publish without label, subscribe label must receive nothing": {
			filters: []testFilter{
				{labels: []Label{{"path", "path1"}}},
			},
			pubs: []testPub{
				{v: "foo"},
				{v: 1},
				{v: []string{"foo2"}},
			},
			expected: []interface{}{},
		},

		"subscribe with (type), (type, label), (type, &&label)": {
			filters: []testFilter{
				{filterType: uint64(9)},
				{labels: []Label{{"xx", "XXX"}}},
				{filterType: "", labels: []Label{{"f1", "F1"}, {"f2", "F2"}}},
			},
			pubs: []testPub{
				{v: uint64(9)},
				{v: []string{"matching label but not type"}, labels: []Label{{"f1", "F1"}, {"f2", "F2"}}},
				{v: "foo", labels: []Label{{"xx", "XXX"}}},
				{v: 1, labels: []Label{{"xx", "XXX"}}},
				{v: "two-label-match", labels: []Label{{"f1", "F1"}, {"f2", "F2"}}},
				{v: "only-one-label-is-no-enough", labels: []Label{{"f1", "f1"}}},
				{v: []string{"with-other-label1", "with-other-label2"}, labels: []Label{{"xx", "other-label"}}},
				{v: []string{"foo1", "foo2"}, labels: []Label{{"xx", "XXX"}}},
			},
			expected: []interface{}{
				uint64(9),
				"foo",
				1,
				"two-label-match",
				[]string{"foo1", "foo2"},
			},
		},
	}
	for s, c := range cases {
		t.Run(s, func(t *testing.T) {
			bus := newRun(t.Name())
			sub := bus.Sub(t.Name())
			for _, f := range c.filters {
				sub.AddFilter(f.filterType, f.labels...)
			}
			sub.Start()
			defer sub.Stop()

			for _, p := range c.pubs {
				bus.Pub(p.v, p.labels...)
			}
			maxDurationTimer := time.NewTimer(5 * time.Millisecond)
			defer maxDurationTimer.Stop()
			received := make([]interface{}, 0)
			go func() {
				for {
					select {
					case i := <-sub.C:
						switch v := i.(type) {
						default:
							received = append(received, v)
						}
					case <-maxDurationTimer.C:
						return
					}
				}
			}()
			<-maxDurationTimer.C
			require.Equal(t, c.expected, received)
		})
	}
}
