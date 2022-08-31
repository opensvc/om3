package pubsub

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	NsCfg = NsAll + 1 + iota
	NsSvcAgg
	NsStatus
)

func newRun(name string) *Bus {
	bus := NewBus(name)
	bus.Start(context.Background())
	return bus
}

func TestPub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	bus.Pub(Publication{Value: "foo", Op: OpCreate})
	bus.Pub(Publication{Value: "foo", Op: OpUpdate})
	bus.Pub(Publication{Value: "foo", Op: OpRead})
	bus.Pub(Publication{Value: "foo", Op: OpDelete})
	bus.Pub(Publication{Value: "bar"})
	bus.Pub(Publication{Value: "foobar"})
}

func TestSubUnSub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	sub := bus.Sub(Subscription{Name: t.Name()}, func(_ interface{}) {})
	bus.Unsub(sub)
}

func TestSubThenPub(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	published := make([]string, 0)
	toPublish := []string{"foo", "foo1", "foo2"}
	bus.Sub(Subscription{Name: t.Name()}, func(s interface{}) {
		published = append(published, s.(string))
	})
	for _, s := range toPublish {
		bus.Pub(Publication{Value: s})
	}
	tr1 := time.NewTimer(time.Microsecond)
	tr2 := time.NewTimer(2 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for {
			select {
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

	var (
		publishedCfg          []uint32
		publishedCfgId1       []uint32
		publishedSvcAgg       []string
		publishedSvcAggDelete []string
	)

	bus.Sub(Subscription{Ns: NsCfg, Matching: "1", Name: "onCfg for Id 1"}, func(s interface{}) {
		t.Logf("-> NsCfg Id 1 sub receive: %v", s)
		publishedCfgId1 = append(publishedCfgId1, s.(uint32))
	})

	bus.Sub(Subscription{Ns: NsCfg, Name: "onCfg"}, func(s interface{}) {
		t.Logf("-> NsCfg sub receive: %v", s)
		publishedCfg = append(publishedCfg, s.(uint32))
	})
	bus.Sub(Subscription{Ns: NsSvcAgg, Name: "onSvcAgg"}, func(s interface{}) {
		t.Logf("-> NsSvcAgg sub receive: %v", s)
		publishedSvcAgg = append(publishedSvcAgg, s.(string))
	})

	bus.Sub(Subscription{Ns: NsSvcAgg, Op: OpDelete, Name: "onSvcAggDelete"}, func(s interface{}) {
		t.Logf("-> NsSvcAgg Op delete sub receive: %v", s)
		publishedSvcAggDelete = append(publishedSvcAggDelete, s.(string))
	})

	t.Log("NsSvcAgg")
	for i, s := range toPublishSvcAgg {
		time.Sleep(1 * time.Nanosecond)
		bus.Pub(Publication{
			Id:    strconv.Itoa(i),
			Value: s,
			Ns:    NsSvcAgg,
		})
	}
	t.Log("NsCfg")
	for i, s := range toPublishCfg {
		time.Sleep(1 * time.Nanosecond)
		bus.Pub(Publication{
			Id:    strconv.Itoa(i),
			Value: s,
			Ns:    NsCfg,
		})
	}
	t.Log("nsCfgDelete")
	for i, s := range toPublishSvcAggDelete {
		time.Sleep(1 * time.Nanosecond)
		bus.Pub(Publication{
			Id:    strconv.Itoa(i),
			Value: s,
			Op:    OpDelete,
			Ns:    NsSvcAgg,
		})
	}
	time.Sleep(1 * time.Millisecond)

	require.ElementsMatch(t, expectedCfg, publishedCfg, "cfg")
	require.ElementsMatch(t, expectedCfgId1, publishedCfgId1, "cfg id1")
	require.ElementsMatch(t, expectedSvcAgg, publishedSvcAgg, "svcAgg")
	require.ElementsMatch(t, expectedSvcAggDelete, publishedSvcAggDelete, "svcAgg delete")
}

func TestSubPubUnSubPubWithoutFilter(t *testing.T) {
	bus := newRun(t.Name())
	defer bus.Stop()
	toPublish := []string{"foo", "foo1", "foo2"}

	var published []string
	id := bus.Sub(Subscription{Name: t.Name()}, func(s interface{}) {
		published = append(published, s.(string))
	})
	for _, s := range toPublish {
		bus.Pub(Publication{Value: s})
	}
	bus.Unsub(id)
	t.Logf("NsAll: %d", NsAll)
	t.Logf("NsCfg: %d", NsCfg)
	t.Logf("NsSvcAgg: %d", NsSvcAgg)
	t.Logf("NsStatus: %d", NsStatus)
	for _, s := range toPublish {
		bus.Pub(Publication{Ns: NsSvcAgg, Value: s})
	}
	tr1 := time.NewTimer(time.Microsecond)
	tr2 := time.NewTimer(2 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for {
			select {
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
