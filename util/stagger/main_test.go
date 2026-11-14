package stagger

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func listen(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

// hang is an attempt that never connects until its context ends.
func hang(ctx context.Context) (net.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func to(addr string) Attempt {
	return func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
}

// An attempt that does not answer does not hold the next one past the delay.
func TestAHangingAttemptDoesNotHoldTheNext(t *testing.T) {
	addr := listen(t)
	begin := time.Now()
	c, err := Dial(context.Background(), 50*time.Millisecond, []Attempt{hang, to(addr)})
	require.NoError(t, err)
	_ = c.Close()
	require.Less(t, time.Since(begin), time.Second)
}

// An attempt failing has the next one started at once, not after the delay.
func TestAFailingAttemptStartsTheNextAtOnce(t *testing.T) {
	addr := listen(t)
	fail := func(context.Context) (net.Conn, error) { return nil, errors.New("refused") }
	begin := time.Now()
	c, err := Dial(context.Background(), time.Hour, []Attempt{fail, to(addr)})
	require.NoError(t, err)
	_ = c.Close()
	require.Less(t, time.Since(begin), time.Second)
}

// No attempt connecting returns the error of the first one, and none at all
// is an error.
func TestNoAttemptConnects(t *testing.T) {
	first := func(context.Context) (net.Conn, error) { return nil, errors.New("first") }
	second := func(context.Context) (net.Conn, error) { return nil, errors.New("second") }
	_, err := Dial(context.Background(), time.Millisecond, []Attempt{first, second})
	require.EqualError(t, err, "first")
	_, err = Dial(context.Background(), time.Millisecond, nil)
	require.Error(t, err)
}
