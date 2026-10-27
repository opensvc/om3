package pgmetrics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/resappsimple"
)

const partsConf = `[DEFAULT]
id = 2f1e5b4c-6c0a-4d63-9a51-6d4c1f7b9d10

[app#a]
type = simple
start = /bin/true

[app#b]
type = simple
start = /bin/true
subset = g1
`

// partsTree installs the object ns1/svc/s1, with app#a out of a subset and
// app#b in the subset g1, and makes the groups of its subset and of app#b,
// app#a's left missing.
func partsTree(t *testing.T) naming.Path {
	t.Helper()
	testhelper.Setup(t)
	p, err := naming.ParsePath("ns1/svc/s1")
	require.NoError(t, err)
	conf := filepath.Join(rawconfig.Paths.Etc, "namespaces", "ns1", "svc", "s1.conf")
	require.NoError(t, os.MkdirAll(filepath.Dir(conf), 0755))
	require.NoError(t, os.WriteFile(conf, []byte(partsConf), 0644))

	host := t.TempDir()
	wasHost := HostCgroupRoot
	HostCgroupRoot = host
	t.Cleanup(func() { HostCgroupRoot = wasHost })
	object := "/opensvc.slice/opensvc-ns.ns1.slice/opensvc-ns.ns1-svc.s1.slice"
	subset := object + "/opensvc-ns.ns1-svc.s1-subset.g1.slice"
	writeCgroupFiles(t, filepath.Join(host, subset))
	writeCgroupFiles(t, filepath.Join(host, subset, "opensvc-ns.ns1-svc.s1-subset.g1-app.b.slice"))
	return p
}

// The subsets and the resources of an object are told from its
// configuration: the subset of a resource is its subset label, and a group
// missing is reported so.
func TestThePartsOfAnObject(t *testing.T) {
	p := partsTree(t)
	var e emitter
	collectParts(&e, p)
	assert.ElementsMatch(t, []string{
		"subset,ns1,ns1/svc/s1,g1,=1",
		"resource,ns1,ns1/svc/s1,,app#a=0",
		"resource,ns1,ns1/svc/s1,g1,app#b=1",
	}, valuesWithAllLabels(e.out, "opensvc_pg_cgroup_exists"))
	assert.ElementsMatch(t, []string{
		"subset,ns1,ns1/svc/s1,g1,=16384",
		"resource,ns1,ns1/svc/s1,g1,app#b=16384",
	}, valuesWithAllLabels(e.out, "opensvc_pg_cgroup_memory_current_bytes"))
}

// An object that is not installed, or has no process group, has no parts.
func TestNoPartsOfAnObjectNotInstalled(t *testing.T) {
	partsTree(t)
	for _, s := range []string{"ns1/svc/other", "ns1/cfg/c", "ns1/sec/s"} {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		var e emitter
		collectParts(&e, p)
		assert.Empty(t, e.out, s)
	}
}

// The page reports the parts of the objects named by the path parameters
// only, and refuses a path it can not parse.
func TestHandlerReportsThePartsAskedFor(t *testing.T) {
	partsTree(t)
	get := func(query string) (int, string) {
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics/pg"+query, nil))
		return rec.Code, rec.Body.String()
	}
	code, body := get("")
	assert.Equal(t, http.StatusOK, code)
	assert.NotContains(t, body, `scope="resource"`)

	code, body = get("?path=ns1/svc/s1")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `opensvc_pg_cgroup_exists{namespace="ns1",path="ns1/svc/s1",rid="app#b",scope="resource",subset="g1"} 1`)
	assert.Contains(t, body, `opensvc_pg_cgroup_exists{namespace="ns1",path="ns1/svc/s1",rid="",scope="subset",subset="g1"} 1`)

	code, _ = get("?path=" + strings.Repeat("/", 3))
	assert.Equal(t, http.StatusBadRequest, code)
}
