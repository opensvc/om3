package object

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/sanswitch"
	"github.com/opensvc/om3/v3/testhelper"
)

// testSwitch reports fixed outputs, as a switch would.
type testSwitch struct {
	sanswitch.Switch
}

func (t *testSwitch) Report(context.Context) (map[string]string, error) {
	return map[string]string{"switchshow": "a", "nsshow": "b", "zoneshow": "c"}, nil
}

func (t *testSwitch) ReportName() string {
	return "sansw1.example.com"
}

var registerTestSwitch sync.Once

// fakeFeeder serves the oc3 switch feed with the status given, and the
// jsonrpc api of the old collector, recording what each received.
type fakeFeeder struct {
	status int

	mu      sync.Mutex
	rest    []map[string]any
	jsonrpc []map[string]any
}

func (f *fakeFeeder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/api/sanswitch":
		f.rest = append(f.rest, m)
		w.WriteHeader(f.status)
	case "/feed/default/call/jsonrpc2":
		f.jsonrpc = append(f.jsonrpc, m)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":0,"id":` + string(mustMarshal(m["id"])) + `}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func setupSwitchNode(t *testing.T, feederURL string) *Node {
	t.Helper()
	registerTestSwitch.Do(func() {
		driver.Register(driver.NewID(driver.GroupSwitch, "testswitch"), func() sanswitch.Driver { return &testSwitch{} })
	})
	testhelper.Setup(t)
	conf := "[node]\nuuid = 00000000-0000-0000-0000-000000000001\n[collector]\nfeeder = " + feederURL + "\n" +
		"[switch#sw1]\ntype = testswitch\n"
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(conf), 0600))
	n, err := NewNode()
	require.NoError(t, err)
	return n
}

// A switch is reported to the oc3 switch feed, under the name its driver
// says the collector knows it by, with the outputs of its commands.
func TestPushSwitchesToOC3(t *testing.T) {
	feeder := &fakeFeeder{status: http.StatusAccepted}
	server := httptest.NewServer(feeder)
	defer server.Close()
	n := setupSwitchNode(t, server.URL)

	l, err := n.PushSwitches(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "oc3", l[0].Via)
	assert.Equal(t, 3, l[0].Commands)
	require.Len(t, feeder.rest, 1)
	assert.Equal(t, "sansw1.example.com", feeder.rest[0]["name"])
	assert.Equal(t, "testswitch", feeder.rest[0]["type"])
	assert.Equal(t, map[string]any{"switchshow": "a", "nsshow": "b", "zoneshow": "c"}, feeder.rest[0]["data"])
	assert.Empty(t, feeder.jsonrpc)
}

// An oc3 older than the switch feed answers 404, and the switch is reported
// through the jsonrpc method v2 used, with the outputs named as v2 named
// them.
func TestPushSwitchesFallsBackToJSONRPC(t *testing.T) {
	feeder := &fakeFeeder{status: http.StatusNotFound}
	server := httptest.NewServer(feeder)
	defer server.Close()
	n := setupSwitchNode(t, server.URL)

	l, err := n.PushSwitches(context.Background(), "switch#sw1")
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "jsonrpc", l[0].Via)
	require.Len(t, feeder.jsonrpc, 1)
	assert.Equal(t, "update_testswitch", feeder.jsonrpc[0]["method"])
	params := feeder.jsonrpc[0]["params"].([]any)
	assert.Equal(t, "sansw1.example.com", params[0])
	assert.Equal(t, []any{"testswitchswitchshow", "testswitchnsshow", "testswitchzoneshow"}, params[1])
	assert.Equal(t, []any{"a", "b", "c"}, params[2])
}

// A switch named that no section declares is an error, rather than a push
// of nothing that looks like a success.
func TestPushSwitchesRefusesAnUnknownSwitch(t *testing.T) {
	feeder := &fakeFeeder{status: http.StatusAccepted}
	server := httptest.NewServer(feeder)
	defer server.Close()
	n := setupSwitchNode(t, server.URL)

	_, err := n.PushSwitches(context.Background(), "nosuchswitch")
	require.Error(t, err)
}
