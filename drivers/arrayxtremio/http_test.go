package arrayxtremio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
)

const (
	testAPIPrefix = "/api/json/v2/types"
	testPassword  = "s3cret"
)

type (
	// fakeXMS is an XtremIO management server holding volumes, ports and
	// mappings in memory, answering the calls the driver makes the way the
	// rest api v2 answers them.
	fakeXMS struct {
		sync.Mutex
		url string

		volumes    map[int]map[string]any
		initiators []map[string]any
		targets    []map[string]any
		lunMaps    map[int]map[string]any
		nextVol    int
		nextMap    int

		// calls is the "<method> <path>" of every call made, in order.
		calls []string

		// bodies is the body of every write, by "<method> <path>".
		bodies map[string][]map[string]any

		// sloppyFilters answers a filtered list with the whole list, as an
		// array ignoring the filter would.
		sloppyFilters bool

		// failLunMapPost answers the creation of a mapping with an error.
		failLunMapPost bool

		// unnamedVolumePost answers the creation of a volume as done, with
		// no link to it.
		unnamedVolumePost bool

		// dropLunMapIndex lists the mappings without their index.
		dropLunMapIndex bool

		// badAuth is set when a call came with credentials other than the
		// ones of the sec.
		badAuth bool
	}
)

func newFakeXMS() *fakeXMS {
	return &fakeXMS{
		volumes: make(map[int]map[string]any),
		lunMaps: make(map[int]map[string]any),
		bodies:  make(map[string][]map[string]any),
		nextVol: 100,
		nextMap: 0,
		initiators: []map[string]any{
			{"name": "host1-hba1", "port-address": "10:00:00:00:00:00:00:01", "ig-id": []any{"ffee", "host1", 3}},
			{"name": "host1-hba2", "port-address": "10:00:00:00:00:00:00:02", "ig-id": []any{"ffef", "host1", 3}},
			{"name": "host2-hba1", "port-address": "10:00:00:00:00:00:00:09", "ig-id": []any{"fff0", "host2", 4}},
		},
		targets: []map[string]any{
			{"name": "X1-SC1-fc1", "port-address": "51:4f:0c:50:00:00:00:01", "tg-id": []any{"aa01", "Default", 1}},
			{"name": "X1-SC2-fc1", "port-address": "51:4f:0c:50:00:00:00:02", "tg-id": []any{"aa01", "Default", 1}},
		},
	}
}

// addVolume puts a volume on the array, of a size in kilobytes.
func (t *fakeXMS) addVolume(index int, name string, kb int64) {
	t.volumes[index] = map[string]any{
		"name":          name,
		"index":         index,
		"naa-name":      fmt.Sprintf("514f0c5000000%03d", index),
		"vol-size":      strconv.FormatInt(kb, 10),
		"creation-time": "2026-10-08 10:00:00",
	}
}

// addLunMap maps a volume, as another run or another host did.
func (t *fakeXMS) addLunMap(volIndex, ig, tg, lun int) int {
	index := t.nextMap
	t.nextMap++
	t.lunMaps[index] = map[string]any{
		"index":     index,
		"vol-name":  t.volumes[volIndex]["name"],
		"vol-index": volIndex,
		"ig-index":  ig,
		"tg-index":  tg,
		"lun":       lun,
	}
	return index
}

func (t *fakeXMS) callsOf(method string) []string {
	t.Lock()
	defer t.Unlock()
	l := make([]string, 0)
	for _, c := range t.calls {
		if strings.HasPrefix(c, method+" ") {
			l = append(l, c)
		}
	}
	return l
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"message": "obj_not_found", "error_code": 400})
}

// filterValue returns the value of a "<field>:eq:<value>" filter on field.
func filterValue(r *http.Request, field string) (string, bool) {
	f := r.URL.Query().Get("filter")
	if !strings.HasPrefix(f, field+":eq:") {
		return "", false
	}
	return strings.TrimPrefix(f, field+":eq:"), true
}

// list answers a list of objects, filtered on field unless the filters are
// sloppy.
func (t *fakeXMS) list(w http.ResponseWriter, r *http.Request, key, field string, objects []map[string]any) {
	value, filtered := filterValue(r, field)
	l := make([]map[string]any, 0)
	for _, o := range objects {
		if filtered && !t.sloppyFilters && fmt.Sprint(o[field]) != value {
			continue
		}
		l = append(l, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{key: l, "links": []any{}})
}

func sortedObjects(m map[int]map[string]any) []map[string]any {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	l := make([]map[string]any, 0, len(m))
	for _, k := range keys {
		l = append(l, m[k])
	}
	return l
}

func (t *fakeXMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.Lock()
	defer t.Unlock()
	if user, pass, ok := r.BasicAuth(); !ok || user != "admin" || pass != testPassword {
		t.badAuth = true
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, testAPIPrefix)
	t.calls = append(t.calls, r.Method+" "+path)
	var body map[string]any
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		_ = json.NewDecoder(r.Body).Decode(&body)
		t.bodies[r.Method+" "+path] = append(t.bodies[r.Method+" "+path], body)
	}
	if r.Method == http.MethodGet && r.URL.Query().Get("cluster-name") != "xt1" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "cluster_not_found"})
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	resource := parts[0]
	var index = -1
	if len(parts) > 1 {
		i, err := strconv.Atoi(parts[1])
		if err != nil {
			notFound(w)
			return
		}
		index = i
	}
	switch {
	case resource == "volumes" && r.Method == http.MethodGet && index >= 0:
		if v, ok := t.volumes[index]; ok {
			writeJSON(w, http.StatusOK, map[string]any{"content": v})
			return
		}
		notFound(w)
	case resource == "volumes" && r.Method == http.MethodGet && r.URL.Query().Get("name") != "":
		name := r.URL.Query().Get("name")
		for _, v := range sortedObjects(t.volumes) {
			if v["name"] == name || t.sloppyFilters {
				writeJSON(w, http.StatusOK, map[string]any{"content": v})
				return
			}
		}
		notFound(w)
	case resource == "volumes" && r.Method == http.MethodGet:
		t.list(w, r, "volumes", "name", sortedObjects(t.volumes))
	case resource == "volumes" && r.Method == http.MethodPost:
		name, _ := body["vol-name"].(string)
		mb, err := strconv.ParseInt(strings.TrimSuffix(fmt.Sprint(body["vol-size"]), "M"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "bad_vol_size"})
			return
		}
		index := t.nextVol
		t.nextVol++
		t.addVolume(index, name, mb*1024)
		if t.unnamedVolumePost {
			writeJSON(w, http.StatusCreated, map[string]any{})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"links": []any{map[string]any{"href": fmt.Sprintf("%s%s/volumes/%d", t.url, testAPIPrefix, index), "rel": "self"}}})
	case resource == "volumes" && r.Method == http.MethodPut && index >= 0:
		v, ok := t.volumes[index]
		if !ok {
			notFound(w)
			return
		}
		mb, err := strconv.ParseInt(strings.TrimSuffix(fmt.Sprint(body["vol-size"]), "M"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "bad_vol_size"})
			return
		}
		v["vol-size"] = strconv.FormatInt(mb*1024, 10)
		writeJSON(w, http.StatusOK, map[string]any{})
	case resource == "volumes" && r.Method == http.MethodDelete && index >= 0:
		if _, ok := t.volumes[index]; !ok {
			notFound(w)
			return
		}
		delete(t.volumes, index)
		writeJSON(w, http.StatusOK, map[string]any{})
	case resource == "initiators" && r.Method == http.MethodGet:
		t.list(w, r, "initiators", "port-address", t.initiators)
	case resource == "targets" && r.Method == http.MethodGet:
		t.list(w, r, "targets", "port-address", t.targets)
	case resource == "lun-maps" && r.Method == http.MethodGet && index >= 0:
		if m, ok := t.lunMaps[index]; ok {
			writeJSON(w, http.StatusOK, map[string]any{"content": m})
			return
		}
		notFound(w)
	case resource == "lun-maps" && r.Method == http.MethodGet:
		l := sortedObjects(t.lunMaps)
		if t.dropLunMapIndex {
			stripped := make([]map[string]any, 0, len(l))
			for _, m := range l {
				c := make(map[string]any)
				for k, v := range m {
					if k != "index" {
						c[k] = v
					}
				}
				stripped = append(stripped, c)
			}
			l = stripped
		}
		t.list(w, r, "lun-maps", "vol-name", l)
	case resource == "lun-maps" && r.Method == http.MethodPost:
		if t.failLunMapPost {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "lun_already_in_use"})
			return
		}
		volIndex, ok := body["vol-id"].(float64)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "vol-id must be an index here"})
			return
		}
		ig, _ := body["ig-id"].(float64)
		tg, _ := body["tg-id"].(float64)
		if _, ok := t.volumes[int(volIndex)]; !ok {
			notFound(w)
			return
		}
		lun := 1
		for _, m := range t.lunMaps {
			if m["ig-index"] == int(ig) {
				lun++
			}
		}
		index := t.addLunMap(int(volIndex), int(ig), int(tg), lun)
		writeJSON(w, http.StatusCreated, map[string]any{"links": []any{map[string]any{"href": fmt.Sprintf("%s%s/lun-maps/%d", t.url, testAPIPrefix, index), "rel": "self"}}})
	case resource == "lun-maps" && r.Method == http.MethodDelete && index >= 0:
		if _, ok := t.lunMaps[index]; !ok {
			notFound(w)
			return
		}
		delete(t.lunMaps, index)
		writeJSON(w, http.StatusOK, map[string]any{})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "no route " + r.Method + " " + path})
	}
}

// newTestEnv sets up a test root holding the sec the password keyword names.
func newTestEnv(t *testing.T, password string) {
	t.Helper()
	testhelper.Setup(t)
	// A sec is encrypted with the cluster secret.
	cfg := &cluster.Config{Name: "cluster1"}
	cfg.SetSecret("070fd9169fc111ec9c5017409407c6ab")
	cluster.ConfigData.Set(cfg)
	p, err := naming.ParsePath("system/sec/xt1")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	require.NoError(t, sec.AddKey("password", []byte(password)))
}

// newTestArray returns an array talking to a stub, configured the way a node
// configuration configures one, its password in a sec.
func newTestArray(t *testing.T, handler http.Handler) (*Array, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	if x, ok := handler.(*fakeXMS); ok {
		x.url = srv.URL
	}
	newTestEnv(t, testPassword)
	return newTestArrayWithPassword(t, srv.URL, "from system/sec/xt1 key password"), srv
}

func newTestArrayWithPassword(t *testing.T, url, password string) *Array {
	t.Helper()
	config := fmt.Sprintf(`
[array#xt1]
type = xtremio
api = %s%s
username = admin
password = %s
`, url, testAPIPrefix, password)
	n, err := object.NewNode(object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)
	a := New()
	a.SetName("array#xt1")
	a.SetConfig(n.MergedConfig())
	return a
}

// toMap renders what an action returns as the collector reads it, from the
// json printed on stdout.
func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

var testMappings = []string{
	"1000000000000001:514f0c5000000001,514f0c5000000002",
	"1000000000000002:514f0c5000000001",
}

// TestAddDisk walks the command the collector queues: the groups are looked
// up before anything is made, the volume is mapped by its index, and the
// report holds the keys v2 reports.
func TestAddDisk(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "other", 1024*1024)
	a, _ := newTestArray(t, x)

	out, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "10g", Mappings: testMappings})
	require.NoError(t, err)
	assert.False(t, x.badAuth)

	// The order of the calls: every port looked up, then the volume made
	// and read back, then mapped, then read again.
	posts := x.callsOf(http.MethodPost)
	assert.Equal(t, []string{"POST /volumes", "POST /lun-maps"}, posts, "two hbas of one initiator group to one target group make one mapping")
	firstPost := -1
	lastLookup := -1
	for i, c := range x.calls {
		if c == "POST /volumes" && firstPost < 0 {
			firstPost = i
		}
		if c == "GET /initiators" || c == "GET /targets" {
			lastLookup = i
		}
	}
	assert.Less(t, lastLookup, firstPost, "the groups are looked up before the volume is made: %v", x.calls)

	vol := x.bodies["POST /volumes"][0]
	assert.Equal(t, "data1", vol["vol-name"])
	assert.Equal(t, "10240M", vol["vol-size"], "10g is 10 GiB")
	assert.Equal(t, "xt1", vol["cluster-id"])
	lm := x.bodies["POST /lun-maps"][0]
	assert.Equal(t, float64(100), lm["vol-id"], "the volume is mapped by the index it was made with")
	assert.Equal(t, float64(3), lm["ig-id"])
	assert.Equal(t, float64(1), lm["tg-id"])

	m := toMap(t, out)
	assert.Equal(t, "514f0c5000000100", m["disk_id"])
	assert.Equal(t, float64(100), m["disk_devid"])
	driverData, ok := m["driver_data"].(map[string]any)
	require.True(t, ok)
	volume, ok := driverData["volume"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "data1", volume["name"])
	assert.Equal(t, "2026-10-08 10:00:00", volume["creation-time"], "the whole object the array describes is kept")
	maps, ok := driverData["mappings"].([]any)
	require.True(t, ok)
	require.Len(t, maps, 1)
	assert.Equal(t, float64(1), maps[0].(map[string]any)["lun"], "a mapping is reported as the array describes it")
	assert.Equal(t, map[string]any{
		"1000000000000001:514f0c5000000001": map[string]any{"hba_id": "1000000000000001", "tgt_id": "514f0c5000000001", "lun": float64(1)},
		"1000000000000001:514f0c5000000002": map[string]any{"hba_id": "1000000000000001", "tgt_id": "514f0c5000000002", "lun": float64(1)},
		"1000000000000002:514f0c5000000001": map[string]any{"hba_id": "1000000000000002", "tgt_id": "514f0c5000000001", "lun": float64(1)},
	}, m["mappings"])
}

// TestAddDiskOutputParsesAsTheCollectorParsesIt runs the command line the
// collector queues and reads its stdout from the first brace.
func TestAddDiskOutputParsesAsTheCollectorParsesIt(t *testing.T) {
	x := newFakeXMS()
	a, _ := newTestArray(t, x)
	var buf strings.Builder
	args := []string{"add", "disk", "--name", "data1", "--size", "1g"}
	for _, m := range testMappings {
		args = append(args, "--mappings", m)
	}
	require.NoError(t, array.RunActions(context.Background(), a.Actions(), args, &buf))
	s := buf.String()
	i := strings.Index(s, "{")
	require.GreaterOrEqual(t, i, 0, s)
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(s[i:]), &m), s)
	for _, k := range []string{"driver_data", "disk_id", "disk_devid", "mappings"} {
		assert.Containsf(t, m, k, "the collector reads %s", k)
	}
}

// TestAddDiskRefusesANumericName pins that a name the driver would read
// back as an index is refused before anything is asked.
func TestAddDiskRefusesANumericName(t *testing.T) {
	x := newFakeXMS()
	a, _ := newTestArray(t, x)
	_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "123", Size: "1g", Mappings: testMappings})
	require.Error(t, err)
	assert.Empty(t, x.calls)
}

// TestAddDiskRefusesAnUnknownPortWithNothingMade pins that a port the array
// does not know fails the command before the volume is made.
func TestAddDiskRefusesAnUnknownPortWithNothingMade(t *testing.T) {
	x := newFakeXMS()
	a, _ := newTestArray(t, x)
	_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "1g", Mappings: []string{"10000000000000ff:514f0c5000000001"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no initiator found")
	assert.Empty(t, x.callsOf(http.MethodPost))
	assert.Empty(t, x.volumes)
}

// TestAddDiskChecksTheGroupLookup pins that an array ignoring the port
// filter does not get the volume exported to the first initiator it lists.
func TestAddDiskChecksTheGroupLookup(t *testing.T) {
	x := newFakeXMS()
	x.sloppyFilters = true
	a, _ := newTestArray(t, x)
	_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "1g", Mappings: []string{"1000000000000009:514f0c5000000002"}})
	require.NoError(t, err)
	lm := x.bodies["POST /lun-maps"][0]
	assert.Equal(t, float64(4), lm["ig-id"], "the initiator group of the port asked for, not of the first one listed")

	x = newFakeXMS()
	x.sloppyFilters = true
	a, _ = newTestArray(t, x)
	_, err = a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "1g", Mappings: []string{"10000000000000ff:514f0c5000000002"}})
	require.Error(t, err)
	assert.Empty(t, x.callsOf(http.MethodPost))
}

// TestAddDiskReportsTheVolumeLeftByAFailedMapping pins that a mapping the
// array refuses is reported with the volume made, which is not deleted.
func TestAddDiskReportsTheVolumeLeftByAFailedMapping(t *testing.T) {
	x := newFakeXMS()
	x.failLunMapPost = true
	a, _ := newTestArray(t, x)
	_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "1g", Mappings: testMappings})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "volume data1 (index 100) was created and is left in place")
	assert.Contains(t, err.Error(), "lun_already_in_use")
	assert.Empty(t, x.callsOf(http.MethodDelete), "the volume made is not deleted")
	assert.Contains(t, x.volumes, 100)
}

// TestAddDiskReportsAVolumeCreatedUnnamed pins that a creation the array
// answers as done, with no link to the volume, is reported as a volume made
// and left in place.
func TestAddDiskReportsAVolumeCreatedUnnamed(t *testing.T) {
	x := newFakeXMS()
	x.unnamedVolumePost = true
	a, _ := newTestArray(t, x)
	_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "1g", Mappings: testMappings})
	require.Error(t, err)
	assert.ErrorIs(t, err, errCreatedUnnamed)
	assert.Contains(t, err.Error(), "volume data1 was created and is left in place, unmapped: check with list volumes --volume data1")
	assert.Equal(t, []string{"POST /volumes"}, x.callsOf(http.MethodPost), "no mapping of a volume not read back")
	assert.Empty(t, x.callsOf(http.MethodDelete))
	assert.Contains(t, x.volumes, 100)
}

// TestAddDiskSizes pins the sizes a new volume is refused.
func TestAddDiskSizes(t *testing.T) {
	x := newFakeXMS()
	a, _ := newTestArray(t, x)
	for _, size := range []string{"", "+1g", "-1g", "1000k", "0"} {
		_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: size})
		assert.Errorf(t, err, "size %q", size)
	}
	assert.Empty(t, x.calls)
	_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "10GB", Mappings: testMappings})
	require.NoError(t, err)
	assert.Equal(t, "10240M", x.bodies["POST /volumes"][0]["vol-size"], "10GB is 10 GiB, as v2 reads it")
}

// TestAddDiskRefusesNoMapping pins that a volume is not made when the
// mappings name no path, as with an empty --mappings: it would be exported
// nowhere, and the success would hide it.
func TestAddDiskRefusesNoMapping(t *testing.T) {
	x := newFakeXMS()
	a, _ := newTestArray(t, x)
	for _, mappings := range [][]string{nil, {""}} {
		_, err := a.AddDisk(context.Background(), OptAddDisk{Name: "data1", Size: "1g", Mappings: mappings})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no mapping")
	}
	assert.Empty(t, x.callsOf(http.MethodPost), "no volume made")
}

// TestDelDisk pins that a volume is unmapped from its own mappings only,
// then deleted by index.
func TestDelDisk(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	x.addVolume(8, "data2", 1024*1024)
	x.addLunMap(7, 3, 1, 1)
	x.addLunMap(8, 3, 1, 2)
	x.addLunMap(7, 4, 1, 1)
	a, _ := newTestArray(t, x)

	out, err := a.DelDisk(context.Background(), "data1")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"disk_id": "514f0c5000000007"}, out)
	assert.Equal(t, []string{"DELETE /lun-maps/0", "DELETE /lun-maps/2", "DELETE /volumes/7"}, x.callsOf(http.MethodDelete))
	assert.Contains(t, x.volumes, 8)
	assert.Contains(t, x.lunMaps, 1)
}

// TestDelDiskIgnoresTheMappingsOfOtherVolumes pins that an array ignoring
// the volume filter on its mappings does not get other disks unexported.
func TestDelDiskIgnoresTheMappingsOfOtherVolumes(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	x.addVolume(8, "data2", 1024*1024)
	x.addLunMap(8, 3, 1, 2)
	x.addLunMap(7, 3, 1, 1)
	x.sloppyFilters = true
	a, _ := newTestArray(t, x)
	_, err := a.DelDisk(context.Background(), "data1")
	require.NoError(t, err)
	assert.Equal(t, []string{"DELETE /lun-maps/1", "DELETE /volumes/7"}, x.callsOf(http.MethodDelete))
}

// TestDelDiskRefusesAVolumeTheArrayDidNotFilter pins that a volume read by
// name is checked to be the one named before it is deleted.
func TestDelDiskRefusesAVolumeTheArrayDidNotFilter(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	x.sloppyFilters = true
	a, _ := newTestArray(t, x)
	_, err := a.DelDisk(context.Background(), "data9")
	require.Error(t, err)
	assert.Empty(t, x.callsOf(http.MethodDelete))
}

// TestDelDiskFailsOnAMappingWithNoIndex pins that a mapping listed without
// an index fails the command before anything is removed, rather than
// removing the mapping of index 0.
func TestDelDiskFailsOnAMappingWithNoIndex(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	x.addVolume(8, "data2", 1024*1024)
	x.addLunMap(8, 3, 1, 2)
	x.addLunMap(7, 3, 1, 1)
	x.dropLunMapIndex = true
	a, _ := newTestArray(t, x)
	_, err := a.DelDisk(context.Background(), "data1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no index")
	assert.Empty(t, x.callsOf(http.MethodDelete))
}

// TestDelDiskByIndex pins that digits name an index, as v2 reads them, and
// that an index another volume is named after is refused.
func TestDelDiskByIndex(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	a, _ := newTestArray(t, x)
	_, err := a.DelDisk(context.Background(), "7")
	require.NoError(t, err)
	assert.Equal(t, []string{"DELETE /volumes/7"}, x.callsOf(http.MethodDelete))

	x = newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	x.addVolume(8, "7", 1024*1024)
	a, _ = newTestArray(t, x)
	_, err = a.DelDisk(context.Background(), "7")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "another volume is named 7")
	assert.Empty(t, x.callsOf(http.MethodDelete))

	_, err = a.DelDisk(context.Background(), "9")
	require.Error(t, err, "an index no volume has")
	assert.Empty(t, x.callsOf(http.MethodDelete))
}

// TestResizeDisk covers the resizes the collector queues.
func TestResizeDisk(t *testing.T) {
	const gib = 1024 * 1024 // in kilobytes, as the array reports a size
	for _, tc := range []struct {
		name     string
		size     string
		truncate bool
		current  int64
		put      string
		wantErr  string
	}{
		{name: "grow", size: "20g", current: 10 * gib, put: "20480M"},
		{name: "grow by", size: "+1g", current: 10 * gib, put: "11264M"},
		{name: "binary units", size: "10GB", current: 5 * gib, put: "10240M"},
		{name: "same size", size: "10g", current: 10 * gib},
		{name: "shrink", size: "5g", current: 10 * gib, wantErr: "drops the end of the volume"},
		{name: "absolute below current", size: "10GB", current: 10*gib + 1024, wantErr: "drops the end of the volume"},
		{name: "shrink allowed", size: "5g", truncate: true, current: 10 * gib, put: "5120M"},
		{name: "negative", size: "-1g", current: 10 * gib, wantErr: "negative"},
		{name: "odd current size", size: "+1g", current: 10*gib + 1, put: "11265M"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newFakeXMS()
			x.addVolume(7, "data1", tc.current)
			a, _ := newTestArray(t, x)
			out, err := a.ResizeDisk(context.Background(), "data1", tc.size, tc.truncate)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Empty(t, x.callsOf(http.MethodPut))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "data1", toMap(t, out)["name"])
			if tc.put == "" {
				assert.Empty(t, x.callsOf(http.MethodPut))
				return
			}
			assert.Equal(t, []string{"PUT /volumes/7"}, x.callsOf(http.MethodPut), "the volume is resized by its index")
			body := x.bodies["PUT /volumes/7"][0]
			assert.Equal(t, tc.put, body["vol-size"])
			assert.Equal(t, "xt1", body["cluster-id"])
		})
	}
}

// TestResizeDiskTruncateOption pins that --truncate reaches the resize.
func TestResizeDiskTruncateOption(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 10*1024*1024)
	a, _ := newTestArray(t, x)
	var buf strings.Builder
	err := array.RunActions(context.Background(), a.Actions(), []string{"resize", "disk", "--volume", "data1", "--size", "5g"}, &buf)
	require.Error(t, err)
	assert.Empty(t, x.callsOf(http.MethodPut))
	err = array.RunActions(context.Background(), a.Actions(), []string{"resize", "disk", "--volume", "data1", "--size", "5g", "--truncate"}, &buf)
	require.NoError(t, err)
	assert.Equal(t, "5120M", x.bodies["PUT /volumes/7"][0]["vol-size"])
}

// TestResizeDiskRefusesAVolumeTheArrayDidNotFilter pins that a volume read
// by name is checked to be the one named before it is resized.
func TestResizeDiskRefusesAVolumeTheArrayDidNotFilter(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	x.sloppyFilters = true
	a, _ := newTestArray(t, x)
	_, err := a.ResizeDisk(context.Background(), "data9", "+1g", false)
	require.Error(t, err)
	assert.Empty(t, x.callsOf(http.MethodPut))
}

// TestAddMapMapsByIndex pins that add map reads the volume and maps it by
// its index.
func TestAddMapMapsByIndex(t *testing.T) {
	x := newFakeXMS()
	x.addVolume(7, "data1", 1024*1024)
	a, _ := newTestArray(t, x)
	_, err := a.AddMap(context.Background(), OptAddMap{Volume: "data1", Mappings: testMappings, LUN: -1})
	require.NoError(t, err)
	require.Len(t, x.bodies["POST /lun-maps"], 1)
	assert.Equal(t, float64(7), x.bodies["POST /lun-maps"][0]["vol-id"])
}

// TestPasswordIsReadFromASec pins that the password keyword names a sec key,
// as v2 reads it, in the current and in the older form, and that a password
// written in clear is refused without being quoted.
func TestPasswordIsReadFromASec(t *testing.T) {
	newTestEnv(t, testPassword)
	for _, ref := range []string{"from system/sec/xt1 key password", "system/sec/xt1"} {
		a := newTestArrayWithPassword(t, "http://127.0.0.1:1", ref)
		got, err := a.password()
		require.NoErrorf(t, err, "password = %s", ref)
		assert.Equal(t, testPassword, got)
	}
	a := newTestArrayWithPassword(t, "http://127.0.0.1:1", "hunter2")
	_, err := a.password()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
}

// TestAReadIsScopedToTheCluster pins that a read names the cluster it is
// about, as v2 names it: one endpoint may serve several.
func TestAReadIsScopedToTheCluster(t *testing.T) {
	var query map[string][]string
	a, _ := newTestArray(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`{"content":{"name":"d1","naa-name":"naa.1","index":7,"vol-size":"1048576"}}`))
	}))
	_, err := a.GetVolume(context.Background(), "d1")
	require.NoError(t, err)
	assert.Equal(t, []string{"xt1"}, query["cluster-name"])
	assert.Equal(t, []string{"1"}, query["full"])
}

// TestTheArrayRefusalIsReported pins that an error the array answers with is
// not read as a volume.
func TestTheArrayRefusalIsReported(t *testing.T) {
	a, _ := newTestArray(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"vol_obj_name_too_long"}`))
	}))
	_, err := a.GetVolume(context.Background(), "d1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vol_obj_name_too_long")
}

// TestALinkOutOfTheAPIIsNotFollowed pins that a link the array answers with
// is followed only to the array itself, the request carrying the password.
func TestALinkOutOfTheAPIIsNotFollowed(t *testing.T) {
	x := newFakeXMS()
	a, srv := newTestArray(t, x)
	for _, link := range []string{
		"http://other.example.com" + testAPIPrefix + "/volumes/1",
		strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + testAPIPrefix + "/volumes/1",
	} {
		_, _, err := a.do(context.Background(), http.MethodGet, link, nil, nil)
		require.Error(t, err, link)
		assert.Contains(t, err.Error(), "refuse to follow the link")
	}
	assert.Empty(t, x.calls, "no request sent")
	_, _, err := a.do(context.Background(), http.MethodGet, srv.URL+testAPIPrefix+"/volumes/100", nil, nil)
	assert.NotContains(t, fmt.Sprint(err), "refuse to follow the link", "a link to the array itself is followed")
}
