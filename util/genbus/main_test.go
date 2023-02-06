package genbus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRefuseStartTwice(t *testing.T) {
	bus := T{}
	defer bus.Stop()
	bus.Start()
	assert.PanicsWithError(t, ErrorStarted.Error(), bus.Start)
}

func TestPanicIfNotStarted(t *testing.T) {
	bus := T{}
	defer bus.Stop()
	t.Run("Post", func(t *testing.T) {
		assert.PanicsWithError(
			t,
			ErrorNeedStart.Error(),
			func() {
				bus.Post(TName("foo"), "app#1", []byte("Warn"), false)
			})
	})
	t.Run("Post", func(t *testing.T) {
		assert.PanicsWithError(
			t,
			ErrorNeedStart.Error(),
			func() {
				bus.Get(TName("foo"), "app#1")
			})
	})
}

func TestPost(t *testing.T) {
	bus := T{}
	bus.Start()
	defer bus.Stop()
	name := TName("root/svc/foo")
	bus.Post(name, "app#1", []byte("status.Down"), false)
	bus.Post(name, "app#2", []byte("status.Up"), false)

	cases := []struct {
		id string
		i  interface{}
	}{
		{"app#1", []byte("status.Down")},
		{"app#2", []byte("status.Up")},
		{"app#3", nil},
	}
	for _, tCase := range cases {
		tc := tCase
		t.Run("ensure Get retrieve expected value for "+tc.id, func(t *testing.T) {
			found := bus.Get(name, tc.id)
			assert.Equal(t, tc.i, found)
		})
	}
	t.Run("status is undef when service is not found", func(t *testing.T) {
		assert.Equal(t, nil, bus.Get(TName(""), ""))
		assert.Equal(t, nil, bus.Get(TName(""), "app#1"))
	})
}

type (
	exampleBus struct {
		*ObjT
		StopFunc func()
	}

	exampleData struct {
		value string
	}
)

func NewExampleBus() *exampleBus {
	ctx, stopFunc := WithContext(context.Background(), "example")
	return &exampleBus{
		ObjT:     FromContext(ctx),
		StopFunc: stopFunc,
	}
}

func (d exampleData) Show() string {
	return "Value is " + d.value
}

//func (b *exampleBus) Get(id string) exampleData {
//	found, ok := b.ObjT.Get(id).(exampleData)
//	if ok {
//		return found
//	}
//	return exampleData{}
//}

//func (b *exampleBus) Wait(id string, d time.Duration) exampleData {
//	found, ok := b.ObjT.Wait(id, d).(exampleData)
//	if ok {
//		return found
//	}
//	return exampleData{}
//}

func TestClusterStatusBus(t *testing.T) {
	ctx, stopper := WithGlobal(context.Background())
	defer stopper()

	exBus := NameBus(ctx, "example")
	data1 := exampleData{"data1"}
	data2 := exampleData{"data2"}
	//nilData := nil
	//nilData := exampleData{}

	//assert.Equal(t, nilData, bus.Get("data#1"))
	assert.Equal(t, nil, exBus.Get("data#1"))
	exBus.Post("data#1", data1, true)
	assert.Equal(t, data1, exBus.Get("data#1"))
	hookId1 := exBus.Register("data#1", func(i interface{}) {
		t.Logf("Value data#1 changed to %v\n", i)
	})

	exBus.Post("data#1", data2, false)
	exBus.Post("data#1", data1, false)
	exBus.Post("data#1", data2, false)
	exBus.Post("data#1", data1, false)

	t.Log("unregister hook")
	exBus.Unregister("data#1", hookId1)
	exBus.Post("data#1", data2, false)
	exBus.Post("data#1", data1, false)
	exBus.Post("data#1", data2, false)
	exBus.Post("data#1", data1, false)

	duration := 40 * time.Millisecond

	go func() {
		time.Sleep(3 * duration)
		exBus.Post("data#2", data2, false)
	}()

	assert.Nil(t, exBus.Get("data#2"))              // not posted yet
	assert.Nil(t, exBus.Wait("data#2", 2*duration)) // not posted yet
	assert.Equal(t, data2, exBus.Wait("data#2", 2*duration))
	assert.Equal(t, data2, exBus.Get("data#2"))

	assert.Equal(t, data2, exBus.Get("data#2"))
	assert.Equal(t, "Value is data2", exBus.Get("data#2").(exampleData).Show())

	exBus2 := NameBus(ctx, "example")
	assert.Equal(t, data2, exBus2.Get("data#2"))

	nsBus1 := NameBus(ctx, "ns1")
	assert.Equal(t, nil, nsBus1.Get("data#2"))
	nsBus1.Post("hb#1.status", "Foo", false)
	assert.Equal(t, "Foo", nsBus1.Get("hb#1.status"))
	assert.Equal(t, nil, exBus.Get("hb#1.status"))
}
