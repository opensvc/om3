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

func TestGetLast(t *testing.T) {
	label1 := Label{"path", "svc1"}
	label2 := Label{"node", "dev2n1"}
	label3 := Label{"cluster", "dev2"}
	bus := newRun(t.Name())
	defer bus.Stop()
	bus.Pub("msg1", label1)
	bus.Pub("msg2", label1, label2)
	bus.Pub("msg3")
	bus.Pub("msg4", label1)
	sub := bus.Sub(t.Name())
	sub.AddFilter("")
	sub.Start()
	defer sub.Stop()
	t.Run("no label", func(t *testing.T) {
		assert.Equal(t, "msg4", sub.GetLast("").(string))
	})
	t.Run("label1", func(t *testing.T) {
		assert.Equal(t, "msg4", sub.GetLast("", label1).(string))
	})
	t.Run("label2", func(t *testing.T) {
		assert.Equal(t, "msg2", sub.GetLast("", label2).(string))
	})
	t.Run("label1 label1", func(t *testing.T) {
		assert.Equal(t, "msg4", sub.GetLast("", label1, label1).(string))
	})
	t.Run("label2 label2", func(t *testing.T) {
		assert.Equal(t, "msg2", sub.GetLast("", label2, label2).(string))
	})
	t.Run("label2 label1", func(t *testing.T) {
		assert.Equal(t, "msg2", sub.GetLast("", label2, label1).(string))
	})
	t.Run("label1 label2", func(t *testing.T) {
		assert.Equal(t, "msg2", sub.GetLast("", label1, label2).(string))
	})
	t.Run("label3", func(t *testing.T) {
		assert.Nil(t, sub.GetLast("", label3))
	})
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
			defer bus.Stop()
			sub := bus.Sub(t.Name())
			for _, f := range c.filters {
				sub.AddFilter(f.filterType, f.labels...)
			}
			sub.Start()
			defer func() {
				assert.NoError(t, sub.Stop())
			}()

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

func TestDropSlowSubscription(t *testing.T) {
	timeout := 10 * time.Millisecond
	for x := 2; x < 5; x++ {
		t.Run(fmt.Sprintf("wait alert is %d x slow duration timeout:%s", x, timeout), func(t *testing.T) {
			waitAlertDuration := timeout * time.Duration(x)
			bus := newRun(t.Name())
			defer bus.Stop()

			t.Log("subscribe on SubscriptionError")
			subAlert := bus.Sub("listen SubscriptionError")
			subAlert.AddFilter(SubscriptionError{})
			subAlert.Start()
			defer func() {
				assert.NoError(t, subAlert.Stop(), "%s stop error", subAlert)
			}()

			queueSize := QueueSize(2)
			t.Log("subscribe with a short timeout, and small queue size")
			slowSub := bus.Sub("listen with short timeout", Timeout(timeout), queueSize)
			slowSub.Start()
			defer func() {
				// ensure stop subscription as been automatically called
				time.Sleep(time.Millisecond)
				assert.ErrorIs(t, slowSub.Stop(), ErrSubscriptionIDNotFound{id: slowSub.id},
					"%s should not exist (it is expected already stopped because dropped)", slowSub)
			}()

			t.Logf("push 'queue size + 2' messages, then read one message => expect one blocking message")
			for i := 0; i < int(queueSize)+2; i++ {
				bus.Pub(i)
			}
			assert.IsType(t, 0, <-slowSub.C, "expected at least one message on %s", slowSub)

			ctx, cancel := context.WithTimeout(context.Background(), waitAlertDuration)
			defer cancel()

			select {
			case i := <-subAlert.C:
				assert.IsTypef(t, SubscriptionError{}, i, "missing message SubscriptionError")
				t.Logf("alert is %s %v", reflect.TypeOf(i), i)
			case <-ctx.Done():
				assert.Nilf(t, ctx.Err(), "SubscriptionError not yet received")
			}
		})
	}
}

func Test_labelMap_Key(t *testing.T) {
	l := labelMap{"a": "a", "b": "b", "c": "c"}
	initialResult := l.Key()
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("iteration %d", i), func(t *testing.T) {
			assert.Equal(t, initialResult, l.Key(), "result must be consistent to avoid subscription leak")
		})
	}
}
