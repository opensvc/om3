package pubsub

import (
	"context"
	"strconv"
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

func TestSubUnsub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	sub := bus.Sub(t.Name())
	sub.Start()
	sub.Stop()
}

func TestSubThenPub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	published := make([]string, 0)
	toPublish := []string{"foo", "foo1", "foo2"}
	sub := bus.Sub(t.Name())
	sub.Start()
	defer sub.Stop()
	for _, s := range toPublish {
		bus.Pub(s)
	}
	tr1 := time.NewTicker(time.Microsecond)
	defer tr1.Stop()
	tr2 := time.NewTicker(2 * time.Millisecond)
	defer tr2.Stop()
	done := make(chan bool)
	go func() {
		for {
			select {
			case i := <-sub.C:
				published = append(published, i.(string))
			case <-tr1.C:
				if len(published) == len(toPublish) {
					done <- true
					return
				}
			case <-tr2.C:
				done <- true
				return
			}
		}
	}()
	<-done
	require.Equal(t, toPublish, published)
}

func TestSubNsThenPub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	expected := []any{"del1", "del2", 1}

	toPublishSvcAgg := []string{"foo", "foo1", "foo2"}
	toPublishSvcAggDelete := []string{"del1", "del2"}
	toPublishCfg := []uint32{0, 1, 2}
	expectedTotal := len(expected)

	var (
		published []any
	)

	sub := bus.Sub(t.Name())
	sub.AddFilter(uint32(0), Label{"ns", "cfg"}, Label{"id", "1"})
	sub.AddFilter("", Label{"ns", "svcagg"}, Label{"op", "delete"})
	sub.Start()
	defer sub.Stop()

	t.Log("NsSvcAgg")
	for i, s := range toPublishSvcAgg {
		time.Sleep(5 * time.Millisecond)
		id := strconv.Itoa(i)
		bus.Pub(
			s,
			Label{"id", id},
			Label{"ns", "svcagg"},
		)
	}
	t.Log("NsCfg")
	for i, s := range toPublishCfg {
		time.Sleep(5 * time.Millisecond)
		bus.Pub(
			s,
			Label{"id", strconv.Itoa(i)},
			Label{"ns", "cfg"},
		)
	}
	t.Log("nsCfgDelete")
	for i, s := range toPublishSvcAggDelete {
		time.Sleep(5 * time.Millisecond)
		bus.Pub(
			s,
			Label{"id", strconv.Itoa(i)},
			Label{"ns", "svcagg"},
			Label{"op", "delete"},
		)
	}
	tr := time.NewTicker(2 * time.Millisecond)
	defer tr.Stop()
	done := make(chan bool)
	recv := 0
	go func() {
		for {
			select {
			case <-tr.C:
				done <- true
				return
			case i := <-sub.C:
				switch c := i.(type) {
				case uint32:
					t.Logf("-> receive uint32: %v", c)
					published = append(published, c)
				case string:
					t.Logf("-> receive string: %v", c)
					published = append(published, c)
				}
			}
			recv += 1
			if recv > expectedTotal {
				done <- true
			}
		}
	}()
	<-done

	require.Contains(t, published, "del1", "")
	require.Contains(t, published, "del2", "")
	require.Contains(t, published, uint32(1), "")
	require.Len(t, published, 3)
}

func TestSubPubWithoutFilter(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	toPublish := []string{"foo", "foo1", "foo2"}

	var published, received []string
	sub := bus.Sub(t.Name())
	sub.Start()
	defer sub.Stop()
	for _, s := range toPublish {
		bus.Pub(s)
		published = append(published, s)
	}
	for _, s := range toPublish {
		bus.Pub(s, Label{"ns", "svcagg"})
		published = append(published, s)
	}
	tr1 := time.NewTicker(time.Microsecond)
	defer tr1.Stop()
	tr2 := time.NewTimer(2 * time.Millisecond)
	defer tr2.Stop()

	done := make(chan bool)
	go func() {
		for {
			select {
			case i := <-sub.C:
				switch c := i.(type) {
				case string:
					received = append(received, c)
				}
			case <-tr1.C:
				if len(published) == len(received) {
					done <- true
					return
				}
			case <-tr2.C:
				done <- true
				return
			}
		}
	}()
	<-done
	require.Equal(t, received, published)
}
