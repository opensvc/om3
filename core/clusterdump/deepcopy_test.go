package clusterdump

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

// The dataset reached by Data.DeepCopy spans a dozen packages and a few
// hundred fields, so these tests drive it by reflection rather than by a
// literal: a hand-written fixture only covers the fields its author
// thought of, and stops covering the fields added after it.
//
// Two properties are asserted. The copy must serialize exactly as the
// json.Marshal followed by json.Unmarshal it replaces, because the
// collector feed and GET /daemon/status put it on the wire. And it must
// share nothing with the dataset it was copied from, which is the whole
// reason the daemon takes a copy.

// marshals reports whether v survives a json round trip. The enum types
// in this tree reject out of range values on the way out, and some also
// reject on the way back in, so both directions decide whether a filler
// value is usable.
func marshals(v reflect.Value) bool {
	b, err := json.Marshal(v.Interface())
	if err != nil {
		return false
	}
	return json.Unmarshal(b, reflect.New(v.Type()).Interface()) == nil
}

// fill writes a distinct non-zero value into everything v reaches, so
// that a field a DeepCopy forgets shows up as a difference rather than
// as two zero values agreeing.
func fill(t testing.TB, v reflect.Value, n *int, depth int) {
	t.Helper()
	if depth > 12 {
		return
	}
	*n++
	seed := *n
	switch v.Kind() {
	case reflect.String:
		v.SetString(fmt.Sprintf("s%d", seed))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(seed))
		if !marshals(v) {
			// An enum. Take its first member that round trips.
			for i := range 64 {
				v.SetInt(int64(i))
				if marshals(v) {
					return
				}
			}
			t.Fatalf("no marshalable value for %s", v.Type())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(seed))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(seed) + 0.5)
	case reflect.Struct:
		switch v.Type() {
		case reflect.TypeOf(time.Time{}):
			v.Set(reflect.ValueOf(time.Unix(int64(1700000000+seed), 0).UTC()))
			return
		case reflect.TypeOf(naming.Path{}):
			// Its fields are not independently valid: the kind has to
			// be one the parser knows.
			p, err := naming.ParsePath(fmt.Sprintf("ns%d/svc/o%d", seed%3, seed))
			if err != nil {
				t.Fatal(err)
			}
			v.Set(reflect.ValueOf(p))
			return
		}
		for i := range v.NumField() {
			if v.Field(i).CanSet() {
				fill(t, v.Field(i), n, depth+1)
			}
		}
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		fill(t, p.Elem(), n, depth+1)
		v.Set(p)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := range 2 {
			fill(t, s.Index(i), n, depth+1)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for i := range 2 {
			k := reflect.New(v.Type().Key()).Elem()
			fill(t, k, n, depth+1)
			if k.Kind() == reflect.String {
				k.SetString(fmt.Sprintf("ns%d/svc/o%d", i, seed))
			}
			e := reflect.New(v.Type().Elem()).Elem()
			fill(t, e, n, depth+1)
			m.SetMapIndex(k, e)
		}
		v.Set(m)
	case reflect.Interface:
		// The any typed fields, nested so that a copy sharing anything
		// below the top level is caught too.
		v.Set(reflect.ValueOf(map[string]any{
			fmt.Sprintf("k%d", seed): []any{"a", float64(seed), map[string]any{"deep": "x"}},
		}))
	}
}

// mutate writes new values in place over everything v reaches, following
// maps, slices and pointers instead of replacing them, so anything a copy
// still shares with v is written through and shows up in the copy.
func mutate(v reflect.Value, n *int) {
	*n++
	seed := *n
	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			v.SetString(fmt.Sprintf("mutated%d", seed))
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.CanSet() {
			was := v.Int()
			v.SetInt(was + int64(seed))
			if !marshals(v) {
				// An enum, now out of range. Leave it be: the mutation
				// has to keep the dataset serializable, and there are
				// plenty of other fields to catch sharing with.
				v.SetInt(was)
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.CanSet() {
			was := v.Uint()
			v.SetUint(was + uint64(seed))
			if !marshals(v) {
				v.SetUint(was)
			}
		}
	case reflect.Float32, reflect.Float64:
		if v.CanSet() {
			v.SetFloat(v.Float() + float64(seed))
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Field(i).CanSet() {
				mutate(v.Field(i), n)
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			mutate(v.Elem(), n)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			mutate(v.Index(i), n)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			// A map value is not addressable: take it out, mutate it,
			// put it back. Its nested slices and maps are written
			// through all the same, which is the point.
			e := reflect.New(v.Type().Elem()).Elem()
			e.Set(v.MapIndex(k))
			mutate(e, n)
			v.SetMapIndex(k, e)
		}
	case reflect.Interface:
		if !v.IsNil() && v.CanSet() {
			e := reflect.New(v.Elem().Type()).Elem()
			e.Set(v.Elem())
			mutate(e, n)
			v.Set(e)
		}
	}
}

func filled(t testing.TB) *Data {
	t.Helper()
	var d Data
	n := 0
	fill(t, reflect.ValueOf(&d).Elem(), &n, 0)
	return &d
}

// jsonDeepCopy is the implementation Data.DeepCopy replaces, kept as the
// reference these tests compare against.
func jsonDeepCopy(t testing.TB, d *Data) *Data {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	n := Data{}
	if err := json.Unmarshal(b, &n); err != nil {
		t.Fatal(err)
	}
	return &n
}

func assertSameJSON(t *testing.T, want, got *Data) {
	t.Helper()
	a, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		return
	}
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			lo := max(0, i-150)
			t.Fatalf("diverges at byte %d:\n json round trip: %s\n deep copy      : %s",
				i, a[lo:min(len(a), i+150)], b[lo:min(len(b), i+150)])
		}
	}
	t.Fatalf("lengths differ: json round trip %d, deep copy %d", len(a), len(b))
}

func TestDeepCopySerializesLikeTheJSONRoundTrip(t *testing.T) {
	d := filled(t)
	assertSameJSON(t, jsonDeepCopy(t, d), d.DeepCopy())
}

// TestDeepCopySerializesLikeTheJSONRoundTripOnEmptyCollections covers what
// the filled dataset cannot: the filler sets every collection, so it never
// exercises nil against empty, and the fields carrying no omitempty
// serialize those two apart, null against [].
//
// The daemonsubsystem collections are left out of the nil-ing below on
// purpose. Their DeepCopy deliberately returns them non-nil, because it
// is the daemon's publication path and not only a copy, so they are the
// one place the round trip and the copy are meant to differ. See
// daemonsubsystem.Heartbeat.DeepCopy.
func TestDeepCopySerializesLikeTheJSONRoundTripOnEmptyCollections(t *testing.T) {
	emptied := func() *Data {
		d := filled(t)
		for path, objectData := range d.Cluster.Object {
			objectData.Scope = nil
			d.Cluster.Object[path] = objectData
		}
		for nodename, nodeData := range d.Cluster.Node {
			nodeData.Os.Paths = nil
			nodeData.Pool = nil
			nodeData.Instance = nil
			d.Cluster.Node[nodename] = nodeData
		}
		d.Cluster.Config.Issues = nil
		d.Cluster.Config.Nodes = nil
		d.Cluster.Config.DNS = nil
		return d
	}
	for name, d := range map[string]*Data{
		"zero":            {},
		"new":             NewData("node1"),
		"nil collections": emptied(),
	} {
		t.Run(name, func(t *testing.T) {
			assertSameJSON(t, jsonDeepCopy(t, d), d.DeepCopy())
		})
	}
}

func TestDeepCopySharesNothingWithTheOriginal(t *testing.T) {
	d := filled(t)
	c := d.DeepCopy()
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	mutate(reflect.ValueOf(d).Elem(), &n)
	after, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("mutated %d values in the original", n)
	if string(before) != string(after) {
		for i := range min(len(before), len(after)) {
			if before[i] != after[i] {
				lo := max(0, i-150)
				t.Fatalf("the copy moved with the original, at byte %d:\n before: %s\n after : %s",
					i, before[lo:min(len(before), i+150)], after[lo:min(len(after), i+150)])
			}
		}
		t.Fatal("the copy moved with the original")
	}
	// Guard against the mutator quietly becoming a no-op, which would
	// have this test pass on a DeepCopy that shares everything.
	mutated, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(mutated) == string(before) {
		t.Fatal("the original did not change: the mutation is not proving anything")
	}
}

// TestDeepCopyOnCapturedDump replays the two properties against a real
// cluster's dataset, which the generated one only approximates. Capture
// one with:
//
//	curl --unix-socket /var/lib/opensvc/lsnr/http.sock \
//	    http://localhost/api/cluster/status > /tmp/dump.json
//	OM_TEST_CLUSTER_DUMP=/tmp/dump.json go test ./core/clusterdump/
func TestDeepCopyOnCapturedDump(t *testing.T) {
	path := os.Getenv("OM_TEST_CLUSTER_DUMP")
	if path == "" {
		t.Skip("set OM_TEST_CLUSTER_DUMP to a captured GET /api/cluster/status body")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d objects, %d nodes", len(d.Cluster.Object), len(d.Cluster.Node))

	// Apply the publication convention the daemonsubsystem methods
	// impose, so the comparison below stays exact and is about the copy.
	// Done here rather than by running the dataset through DeepCopy,
	// which would launder a dropped field past the very check that is
	// meant to catch it. A dump taken from a daemon that already
	// normalizes is unaffected.
	for nodename, nodeData := range d.Cluster.Node {
		if nodeData.Daemon.Dns.Nameservers == nil {
			nodeData.Daemon.Dns.Nameservers = []string{}
		}
		if nodeData.Daemon.Heartbeat.LastMessages == nil {
			nodeData.Daemon.Heartbeat.LastMessages = []daemonsubsystem.HeartbeatLastMessage{}
		}
		if nodeData.Daemon.Heartbeat.Streams == nil {
			nodeData.Daemon.Heartbeat.Streams = []daemonsubsystem.HeartbeatStream{}
		}
		for i, stream := range nodeData.Daemon.Heartbeat.Streams {
			if stream.Alerts == nil {
				nodeData.Daemon.Heartbeat.Streams[i].Alerts = []daemonsubsystem.Alert{}
			}
			if stream.Peers == nil {
				nodeData.Daemon.Heartbeat.Streams[i].Peers = map[string]daemonsubsystem.HeartbeatStreamPeerStatus{}
			}
		}
		d.Cluster.Node[nodename] = nodeData
	}

	assertSameJSON(t, jsonDeepCopy(t, &d), d.DeepCopy())
}

// benchData is the generated dataset, or the captured one when
// OM_TEST_CLUSTER_DUMP points at it, since a real cluster is both much
// larger and differently shaped.
func benchData(b *testing.B) *Data {
	b.Helper()
	path := os.Getenv("OM_TEST_CLUSTER_DUMP")
	if path == "" {
		return filled(b)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		b.Fatal(err)
	}
	b.Logf("%d objects, %d nodes", len(d.Cluster.Object), len(d.Cluster.Node))
	return &d
}

func BenchmarkDeepCopy(b *testing.B) {
	d := benchData(b)
	b.ResetTimer()
	for b.Loop() {
		_ = d.DeepCopy()
	}
}

func BenchmarkDeepCopyJSONRoundTrip(b *testing.B) {
	d := benchData(b)
	b.ResetTimer()
	for b.Loop() {
		_ = jsonDeepCopy(b, d)
	}
}
