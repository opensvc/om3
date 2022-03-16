package pubsub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func newRun(name string) (*T, chan<- interface{}) {
	p := T{}
	cmdC, err := p.Run(context.Background(), name)
	if err != nil {
		return nil, nil
	}
	return &p, cmdC
}
func TestRefuseRunTwice(t *testing.T) {
	p := T{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := p.Run(ctx, t.Name())
	require.Nil(t, err)
	_, err = p.Run(ctx, t.Name())
	require.ErrorIs(t, err, ErrorAlreadyRunning)
}

func TestRunStopRun(t *testing.T) {
	p := T{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := p.Run(ctx, t.Name())
	require.Nil(t, err)
	p.Stop()
	_, err = p.Run(ctx, t.Name())
	require.Nil(t, err)
}

func TestRunCancelRun(t *testing.T) {
	p := T{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := p.Run(ctx, t.Name())
	require.Nil(t, err)
	cancel()
	p.WaitStopped()
	_, err = p.Run(ctx, t.Name())
	require.Nil(t, err)
}

func TestPub(t *testing.T) {
	p, cmdC := newRun(t.Name())
	defer p.Stop()
	Pub(cmdC, "foo")
	Pub(cmdC, "bar")
	Pub(cmdC, "foobar")
}

func TestSubUnSub(t *testing.T) {
	p, cmdC := newRun(t.Name())
	defer p.Stop()
	sub := Sub(cmdC, t.Name(), func(_ interface{}) {})
	Unsub(cmdC, sub)
}

func TestSubThenPub(t *testing.T) {
	p, cmdC := newRun(t.Name())
	defer p.Stop()
	published := []string{}
	toPublish := []string{"foo", "foo1", "foo2"}
	Sub(cmdC, t.Name(), func(s interface{}) {
		published = append(published, s.(string))
	})
	for _, s := range toPublish {
		Pub(cmdC, s)
	}
	require.Equal(t, published, toPublish)
}

func TestSubPubUnSubPub(t *testing.T) {
	p, cmdC := newRun(t.Name())
	defer p.Stop()
	published := []string{}
	toPublish := []string{"foo", "foo1", "foo2"}
	id := Sub(cmdC, t.Name(), func(s interface{}) {
		published = append(published, s.(string))
	})
	for _, s := range toPublish {
		Pub(cmdC, s)
	}
	Unsub(cmdC, id)
	for _, s := range toPublish {
		Pub(cmdC, s)
	}
	require.Equal(t, published, toPublish)
}
