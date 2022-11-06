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

func TestSubUnSub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	sub := bus.Sub(t.Name(), nil)
	sub.Stop()
}

func TestSubThenPub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	published := make([]string, 0)
	toPublish := []string{"foo", "foo1", "foo2"}
	sub := bus.Sub(t.Name(), nil)
	defer sub.Stop()
	for _, s := range toPublish {
		bus.Pub(s)
	}
	tr1 := time.NewTimer(time.Microsecond)
	tr2 := time.NewTimer(2 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for {
			select {
			case i := <-sub.C:
				published = append(published, i.(string))
			case <-tr1.C:
				if len(published) != len(toPublish) {
					tr1.Reset(time.Microsecond)
				} else {
					if !tr2.Stop() {
						<-tr2.C
					}
					done <- true
					return
				}
			case <-tr2.C:
				if !tr1.Stop() {
					<-tr1.C
				}
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
	expectedCfg := []uint32{1, 20, 30}
	expectedCfgId1 := []uint32{20}
	expectedSvcAgg := []string{"foo", "foo1", "foo2", "foo1", "foo2"}
	expectedSvcAggDelete := []string{"foo1", "foo2"}

	toPublishSvcAgg := []string{"foo", "foo1", "foo2"}
	toPublishSvcAggDelete := []string{"foo1", "foo2"}
	toPublishCfg := []uint32{1, 20, 30}
	expectedTotal := len(expectedCfg) + len(expectedCfgId1) + len(expectedSvcAgg) + len(expectedSvcAggDelete)

	var (
		publishedCfg          []uint32
		publishedCfgId1       []uint32
		publishedSvcAgg       []string
		publishedSvcAggDelete []string
	)

	subCfgId1 := bus.Sub("onCfg for Id 1", 0, Label{"ns", "cfg"}, Label{"id", "1"})
	defer subCfgId1.Stop()

	subCfg := bus.Sub("onCfg", 0, Label{"ns", "cfg"})
	defer subCfg.Stop()

	subSvcAgg := bus.Sub("onSvcAgg", "", Label{"ns", "svcagg"})
	defer subSvcAgg.Stop()

	subSvcAggDelete := bus.Sub("onSvcAggDelete", "", Label{"ns", "svcagg"}, Label{"op", "delete"})
	defer subSvcAggDelete.Stop()

	t.Log("NsSvcAgg")
	for i, s := range toPublishSvcAgg {
		time.Sleep(1 * time.Nanosecond)
		id := strconv.Itoa(i)
		bus.Pub(
			s,
			Label{"id", id},
			Label{"ns", "svcagg"},
		)
	}
	t.Log("NsCfg")
	for i, s := range toPublishCfg {
		time.Sleep(1 * time.Nanosecond)
		bus.Pub(
			s,
			Label{"id", strconv.Itoa(i)},
			Label{"ns", "cfg"},
		)
	}
	t.Log("nsCfgDelete")
	for i, s := range toPublishSvcAggDelete {
		time.Sleep(1 * time.Nanosecond)
		bus.Pub(
			s,
			Label{"id", strconv.Itoa(i)},
			Label{"ns", "svcagg"},
			Label{"op", "delete"},
		)
	}
	tr := time.NewTimer(2 * time.Millisecond)
	done := make(chan bool)
	recv := 0
	go func() {
		for {
			select {
			case <-tr.C:
				done <- true
				return
			case i := <-subCfgId1.C:
				t.Logf("-> NsCfg Id 1 sub receive: %v", i)
				publishedCfgId1 = append(publishedCfgId1, i.(uint32))
			case i := <-subCfg.C:
				t.Logf("-> NsCfg sub receive: %v", i)
				publishedCfg = append(publishedCfg, i.(uint32))
			case i := <-subSvcAgg.C:
				t.Logf("-> NsSvcAgg sub receive: %v", i)
				publishedSvcAgg = append(publishedSvcAgg, i.(string))
			case i := <-subSvcAggDelete.C:
				t.Logf("-> NsSvcAgg Op delete sub receive: %v", i)
				publishedSvcAggDelete = append(publishedSvcAggDelete, i.(string))
			}
			recv += 1
			if recv >= expectedTotal {
				done <- true
			}
		}
	}()
	<-done
	if !tr.Stop() {
		<-tr.C
	}

	require.ElementsMatch(t, expectedCfg, publishedCfg, "cfg")
	require.ElementsMatch(t, expectedCfgId1, publishedCfgId1, "cfg id1")
	require.ElementsMatch(t, expectedSvcAgg, publishedSvcAgg, "svcAgg")
	require.ElementsMatch(t, expectedSvcAggDelete, publishedSvcAggDelete, "svcAgg delete")
}

func TestSubPubWithoutFilter(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	toPublish := []string{"foo", "foo1", "foo2"}

	var published, received []string
	sub := bus.Sub(t.Name(), nil)
	defer sub.Stop()
	onSub := func(s any) {
		received = append(received, s.(string))
	}
	for _, s := range toPublish {
		bus.Pub(s)
		published = append(published, s)
	}
	for _, s := range toPublish {
		bus.Pub(s, Label{"ns", "svcagg"})
		published = append(published, s)
	}
	tr1 := time.NewTimer(time.Microsecond)
	tr2 := time.NewTimer(2 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for {
			select {
			case i := <-sub.C:
				onSub(i)
			case <-tr1.C:
				if len(published) != len(toPublish) {
					tr1.Reset(time.Microsecond)
				} else {
					if !tr2.Stop() {
						<-tr2.C
					}
					done <- true
					return
				}
			case <-tr2.C:
				if !tr1.Stop() {
					<-tr1.C
				}
				done <- true
				return
			}
		}
	}()
	<-done
	require.Equal(t, received, published)
}
