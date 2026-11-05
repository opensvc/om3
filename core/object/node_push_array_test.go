package object

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/util/httphelper"
)

// arrayFeeder serves the oc3 array feed with the status given, recording
// the requests it received.
type arrayFeeder struct {
	status   int
	path     string
	encoding string
	body     map[string]any
	calls    int
}

func (f *arrayFeeder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls++
	f.path = r.URL.Path
	f.encoding = r.Header.Get("Content-Encoding")
	if zr, err := gzip.NewReader(r.Body); err == nil {
		_ = json.NewDecoder(zr).Decode(&f.body)
	}
	w.WriteHeader(f.status)
	if f.status >= 400 {
		_, _ = w.Write([]byte(`{"text":"the collector does not parse quiet arrays yet"}`))
	}
}

func newArrayFeeder(t *testing.T, status int) (*arrayFeeder, *httphelper.T) {
	t.Helper()
	f := &arrayFeeder{status: status}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	cli, err := collector.NewRequester(srv.URL, "", false)
	require.NoError(t, err)
	return f, cli
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

// An array is posted to the oc3 array feed, compressed, with its name, its
// type, and the sections of its driver by name.
func TestAnArrayIsPostedToTheFeed(t *testing.T) {
	feeder, cli := newArrayFeeder(t, http.StatusAccepted)
	push := ArrayPush{Name: "arr1", Type: "pure"}

	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), cli, &fakeArray{}, ArrayItem{Name: "arr1", Type: "pure"}, &push)
	require.NoError(t, err)

	assert.Equal(t, "/api/array", feeder.path)
	assert.Equal(t, "gzip", feeder.encoding)
	assert.Equal(t, map[string]any{
		"name": "arr1",
		"type": "pure",
		"data": map[string]any{
			"system": map[string]any{"model": "m1"},
			"pools":  []any{"p1", "p2"},
		},
	}, feeder.body)
	assert.Equal(t, 2, push.Keys)
}

// A refusal of the collector is an error carrying its reason.
func TestAnArrayRefusedSaysWhy(t *testing.T) {
	_, cli := newArrayFeeder(t, http.StatusBadRequest)
	push := ArrayPush{}

	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), cli, &fakeArray{}, ArrayItem{Name: "arr1", Type: "quiet"}, &push)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[400]: the collector does not parse quiet arrays yet")
}

// TestAnArrayThatCannotBeReadIsNotPushed pins that half a configuration is
// never reported as a whole one: the collector would take what is missing for
// what has gone away.
func TestAnArrayThatCannotBeReadIsNotPushed(t *testing.T) {
	feeder, cli := newArrayFeeder(t, http.StatusAccepted)
	push := ArrayPush{}

	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), cli, &fakeArray{fail: true}, ArrayItem{Name: "arr1", Type: "pure"}, &push)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system")
	assert.Zero(t, feeder.calls, "nothing must be pushed")
}

// TestADriverThatDoesNotReportSaysSo pins the message for a driver that has no
// Reports of its own yet.
func TestADriverThatDoesNotReportSaysSo(t *testing.T) {
	feeder, cli := newArrayFeeder(t, http.StatusAccepted)
	push := ArrayPush{}
	node := Node{}
	err := node.pushArrayWithDriver(context.Background(), cli, &silentArray{}, ArrayItem{Name: "arr1", Type: "quiet"}, &push)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not report its configuration")
	assert.Zero(t, feeder.calls)
}

type silentArray struct{ array.Array }

func (t *silentArray) Run([]string) error { return nil }
