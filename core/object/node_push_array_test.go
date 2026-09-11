package object

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"

	"github.com/opensvc/om3/v3/core/array"
)

// callSpy records what an array pushed, without a collector to push to.
type callSpy struct {
	method string
	params []interface{}
	err    error
}

func (t *callSpy) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	t.method = method
	t.params = params
	return &jsonrpc.RPCResponse{}, t.err
}

// fakeArray is an array driver reporting two sections.
type fakeArray struct {
	array.Array
	fail bool
}

func (t *fakeArray) Run([]string) error { return nil }

func (t *fakeArray) Reports() []array.Report {
	return []array.Report{
		{
			Key: "system",
			Get: func(context.Context) (any, error) {
				if t.fail {
					return nil, errors.New("array unreachable")
				}
				return map[string]string{"model": "m1"}, nil
			},
		},
		{
			Key: "pools",
			Get: func(context.Context) (any, error) {
				return []string{"p1", "p2"}, nil
			},
		},
	}
}

// TestAnArrayIsPushedAsV2PushesIt pins the shape of the call: the method the
// collector exports for the type of the array, then its name, the names of its
// sections and their contents. That is what v2 sends, and the collector reads
// it by position.
func TestAnArrayIsPushedAsV2PushesIt(t *testing.T) {
	spy := &callSpy{}
	drv := &fakeArray{}
	push := ArrayPush{Name: "array#arr1", Type: "freenas"}

	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), spy, drv, ArrayItem{Name: "array#arr1", Type: "freenas"}, &push)
	require.NoError(t, err)

	assert.Equal(t, "update_freenas", spy.method)
	require.Len(t, spy.params, 3)
	assert.Equal(t, "array#arr1", spy.params[0])
	assert.Equal(t, []string{"system", "pools"}, spy.params[1])
	assert.Equal(t, []any{map[string]string{"model": "m1"}, []string{"p1", "p2"}}, spy.params[2])
	assert.Equal(t, 2, push.Keys)
}

// TestAnArrayThatCannotBeReadIsNotPushed pins that half a configuration is
// never reported as a whole one: the collector would take what is missing for
// what has gone away.
func TestAnArrayThatCannotBeReadIsNotPushed(t *testing.T) {
	spy := &callSpy{}
	drv := &fakeArray{fail: true}
	push := ArrayPush{}

	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), spy, drv, ArrayItem{Name: "array#arr1", Type: "freenas"}, &push)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system")
	assert.Empty(t, spy.method, "nothing must be pushed")
}

// TestADriverThatDoesNotReportSaysSo pins the message for a driver that has no
// Reports of its own yet.
func TestADriverThatDoesNotReportSaysSo(t *testing.T) {
	spy := &callSpy{}
	push := ArrayPush{}
	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), spy, &silentArray{}, ArrayItem{Name: "array#arr1", Type: "quiet"}, &push)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not report its configuration")
	assert.Empty(t, spy.method)
}

type silentArray struct{ array.Array }

func (t *silentArray) Run([]string) error { return nil }
