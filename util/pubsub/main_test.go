package pubsub

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRun(name string) *Bus {
	bus := NewBus(name)
	bus.SetDefaultSubscriptionQueueSize(200)
	bus.Start(context.Background())
	return bus
}

type (
	msgT struct {
		Msg
		v interface{}
	}

	msgS struct {
		Msg
		v string
	}

	msgI struct {
		Msg
		v uint64
	}

	Valuer interface {
		Value() interface{}
	}
)

func (m *msgT) Value() interface{} {
	return m.v
}

func (m *msgS) Value() interface{} {
	return m.v
}

func (m *msgI) Value() interface{} {
	return m.v
}

func TestPub(t *testing.T) {
	bus := newRun(t.Name())
	ctx := ContextWithBus(context.Background(), bus)

	defer bus.Stop()
	pub := PubFromContext(ctx)
	pub.Pub(&msgT{v: "foo"}, Label{"op", "create"})
	pub.Pub(&msgT{v: "foo"}, Label{"op", "update"})
	pub.Pub(&msgT{v: "foo"}, Label{"op", "read"})
	pub.Pub(&msgT{v: "foo"}, Label{"op", "delete"})
	pub.Pub(&msgT{v: "bar"})
	pub.Pub(&msgT{v: "foobar"})
}

func TestGenerateCombinations(t *testing.T) {
	cases := map[string]struct {
		input    []string
		expected [][]string
	}{
		"empty input": {
			input:    []string{},
			expected: [][]string{},
		},
		"single element": {
			input:    []string{"a"},
			expected: [][]string{{"a"}},
		},
		"two elements": {
			input:    []string{"a", "b"},
			expected: [][]string{{"a"}, {"b"}, {"a", "b"}},
		},
		"three elements": {
			input:    []string{"a", "b", "c"},
			expected: [][]string{{"a"}, {"b"}, {"a", "b"}, {"c"}, {"a", "c"}, {"b", "c"}, {"a", "b", "c"}},
		},
		"duplicate elements": {
			input:    []string{"a", "a"},
			expected: [][]string{{"a"}, {"a"}, {"a", "a"}},
		},
	}

	for name, c := range cases {
		t.Run("combinations "+name, func(t *testing.T) {
			result := combinations(c.input)
			assert.ElementsMatch(t, c.expected, result)
		})
	}
}
func TestSub(t *testing.T) {
	type (
		testPub struct {
			v      Messager
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
				{v: &msgT{v: "foo"}},
				{v: &msgT{v: "pub with label"}, labels: []Label{{"xx", "XXX"}}},
				{v: &msgT{v: "foo2"}},
				{v: &msgT{v: 1}},
			},
			expected: []interface{}{"foo", "pub with label", "foo2", 1},
		},

		"publish without label, subscribe label must receive nothing": {
			filters: []testFilter{
				{labels: []Label{{"path", "path1"}}},
			},
			pubs: []testPub{
				{v: &msgT{v: "foo"}},
				{v: &msgT{v: 1}},
				{v: &msgT{v: []string{"foo2"}}},
			},
			expected: []interface{}{},
		},

		"subscribe with (type), (type, label), (type, &&label)": {
			filters: []testFilter{
				{filterType: &msgI{v: 9}},
				{labels: []Label{{"xx", "XXX"}}},
				{filterType: &msgS{}, labels: []Label{{"f1", "F1"}, {"f2", "F2"}}},
			},
			pubs: []testPub{
				{v: &msgI{v: uint64(9)}},
				{v: &msgT{v: []string{"matching label but not type"}}, labels: []Label{{"f1", "F1"}, {"f2", "F2"}}},
				{v: &msgS{v: "foo"}, labels: []Label{{"xx", "XXX"}}},
				{v: &msgT{v: 1}, labels: []Label{{"xx", "XXX"}}},
				{v: &msgS{v: "two-label-match"}, labels: []Label{{"f1", "F1"}, {"f2", "F2"}}},
				{v: &msgS{v: "only-one-label-is-no-enough"}, labels: []Label{{"f1", "f1"}}},
				{v: &msgT{v: []string{"with-other-label1", "with-other-label2"}}, labels: []Label{{"xx", "other-label"}}},
				{v: &msgT{v: []string{"foo1", "foo2"}}, labels: []Label{{"xx", "XXX"}}},
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
			defer bus.Stop()
			sub := bus.Sub(t.Name())
			for _, f := range c.filters {
				sub.AddFilter(f.filterType, f.labels...)
			}
			sub.Start()
			defer func() {
				assert.NoError(t, sub.Stop())
			}()

			var publisher Publisher = bus
			for _, p := range c.pubs {
				publisher.Pub(p.v, p.labels...)
			}
			maxDurationTimer := time.NewTimer(5 * time.Millisecond)
			defer maxDurationTimer.Stop()
			receivedC := make(chan []interface{})
			go func() {
				received := make([]interface{}, 0)
				defer func() {
					receivedC <- received
				}()
				for {
					select {
					case i := <-sub.C:
						switch v := i.(type) {
						case Valuer:
							received = append(received, v.Value())
						default:
							received = append(received, v)
						}
					case <-maxDurationTimer.C:
						return
					}
				}
			}()
			require.Equal(t, c.expected, <-receivedC)
			//r := <-receivedC
			//for i, v := range c.expected {
			//	require.Equal(t, v, r[i].(Valuer).Value())
			//}
		})
	}
}

func TestDropSlowSubscription(t *testing.T) {
	timeout := 50 * time.Millisecond
	for x := 3; x < 5; x++ {
		t.Run(fmt.Sprintf("wait alert is %d x slow duration timeout:%s", x, timeout), func(t *testing.T) {
			waitAlertDuration := timeout * time.Duration(x)
			bus := newRun(t.Name())
			defer bus.Stop()
			var publisher Publisher = bus
			t.Log("subscribe on SubscriptionError")
			subAlert := bus.Sub("listen SubscriptionError")
			subAlert.AddFilter(&SubscriptionError{})
			subAlert.Start()
			defer func() {
				assert.NoError(t, subAlert.Stop(), "%s stop error", subAlert)
			}()

			queueSize := WithQueueSize(2)
			t.Log("subscribe with a short timeout, and small queue size")
			slowSub := bus.Sub("listen with short timeout", Timeout(timeout), queueSize)
			slowSub.AddFilter(&msgT{})
			slowSub.Start()
			defer func() {
				// ensure stop subscription as been automatically called
				time.Sleep(time.Millisecond)
				assert.ErrorIs(t, slowSub.Stop(), ErrSubscriptionIDNotFound{id: slowSub.id},
					"%s should not exist (it is expected already stopped because dropped)", slowSub)
			}()

			t.Logf("push 'queue size + 2' messages, then read one message => expect one blocking message")
			for i := 0; i < int(queueSize)+2; i++ {
				publisher.Pub(&msgT{v: i})
			}
			assert.IsType(t, &msgT{}, <-slowSub.C, "expected at least one message on %s", slowSub)

			ctx, cancel := context.WithTimeout(context.Background(), waitAlertDuration)
			defer cancel()

			select {
			case i := <-subAlert.C:
				assert.IsTypef(t, &SubscriptionError{}, i, "missing message SubscriptionError")
				t.Logf("alert is %s %v", reflect.TypeOf(i), i)
			case <-ctx.Done():
				assert.Nilf(t, ctx.Err(), "SubscriptionError not yet received")
			}
		})
	}
}

func Test_labelMap_Key(t *testing.T) {
	l := Labels{"a": "a", "b": "b", "c": "c"}
	initialResult := l.Key()
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("iteration %d", i), func(t *testing.T) {
			assert.Equal(t, initialResult, l.Key(), "result must be consistent to avoid subscription leak")
		})
	}
}

func TestLabelsKeys(t *testing.T) {
	cases := map[string]struct {
		labels   Labels
		expected []string
	}{
		"empty labels": {
			labels:   Labels{},
			expected: []string{""},
		},
		"single key-value pair": {
			labels:   Labels{"a": "1"},
			expected: []string{"", "{a=1}"},
		},
		"two key-value pairs": {
			labels:   Labels{"a": "1", "b": "2"},
			expected: []string{"", "{a=1}", "{a=1}{b=2}", "{b=2}"},
		},
		"three key-value pairs": {
			labels: Labels{"a": "1", "b": "2", "c": "3"},
			expected: []string{
				"",
				"{a=1}", "{b=2}", "{c=3}",
				"{a=1}{b=2}", "{a=1}{c=3}", "{b=2}{c=3}",
				"{a=1}{b=2}{c=3}",
			},
		},
		"four key-value pairs": {
			labels: Labels{"a": "1", "b": "2", "c": "3", "d": "4"},
			expected: []string{
				"",
				"{a=1}", "{b=2}", "{c=3}", "{d=4}",
				"{a=1}{b=2}", "{a=1}{c=3}", "{a=1}{d=4}", "{b=2}{c=3}", "{b=2}{d=4}", "{c=3}{d=4}",
				"{a=1}{b=2}{c=3}", "{a=1}{b=2}{d=4}", "{a=1}{c=3}{d=4}", "{b=2}{c=3}{d=4}",
				"{a=1}{b=2}{c=3}{d=4}",
			},
		},
		"keys with equal values": {
			labels: Labels{"x": "val", "y": "val"},
			expected: []string{
				"",
				"{x=val}", "{y=val}",
				"{x=val}{y=val}",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			result := c.labels.Keys()
			assert.ElementsMatchf(t, c.expected, result, "Keys() output mismatch:\nexpected: %s\nactual:   %s", c.expected, result)
		})
	}
}

func TestSubscriptionKeys(t *testing.T) {
	cases := map[string]struct {
		filters  filters
		expected []string
	}{
		"no filters": {
			filters:  filters{},
			expected: []string{":"},
		},
		"single filter": {
			filters: filters{
				filter{
					labels:   map[string]string{"a": "1"},
					dataType: "type1",
				},
			},
			expected: []string{"type1:{a=1}"},
		},
		"multiple filters unsorted must return filter with sorted labels to match combination": {
			filters: filters{
				filter{
					labels:   map[string]string{"c": "3", "b": "2", "a": "1"},
					dataType: "type1",
				},
			},
			expected: []string{"type1:{a=1}{b=2}{c=3}"},
		},
		"multiple datatype and filters unsorted": {
			filters: filters{
				filter{
					labels:   map[string]string{"C": "3", "B": "2", "A": "1"},
					dataType: "type3",
				},
				filter{
					labels:   map[string]string{"c": "3", "b": "2", "a": "1"},
					dataType: "type1",
				},
			},
			expected: []string{"type1:{a=1}{b=2}{c=3}", "type3:{A=1}{B=2}{C=3}"},
		},
		"multiple datatype and filters unsorted with same key": {
			filters: filters{
				filter{
					labels:   map[string]string{"C": "3", "B": "2", "A": "1"},
					dataType: "type3",
				},
				filter{
					labels:   map[string]string{"C": "5", "B": "2", "A": "1"},
					dataType: "type3",
				},
			},
			expected: []string{"type3:{A=1}{B=2}{C=3}", "type3:{A=1}{B=2}{C=5}"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			sub := &Subscription{
				filters: c.filters,
			}
			result := sub.keys()
			assert.Equal(t, c.expected, result, "keys() output mismatch")
		})
	}
}
