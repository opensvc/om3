package arrayxtremio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"
)

// newTestArray returns an array talking to a stub, configured the way a node
// configuration configures one.
func newTestArray(t *testing.T, handler http.HandlerFunc) (*Array, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	config := fmt.Sprintf(`
[array#xt1]
type = xtremio
api = %s/api/json/v2/types
username = admin
password = secret
`, srv.URL)
	n, err := object.NewNode(object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)

	a := New()
	a.SetName("array#xt1")
	a.SetConfig(n.MergedConfig())
	return a, srv
}

// TestAResizeAsksTheArrayWhatV2Asks walks a whole command through the http
// layer: the volume is named the way the array reads a name, the size is sent
// in megabytes, and the array is scoped to the cluster.
func TestAResizeAsksTheArrayWhatV2Asks(t *testing.T) {
	var (
		gotPut  map[string]any
		putPath string
		putQ    map[string][]string
	)
	a, _ := newTestArray(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putPath = r.URL.Path
			putQ = r.URL.Query()
			_ = json.NewDecoder(r.Body).Decode(&gotPut)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"content":{"name":"d1","naa-name":"naa.1","index":7,"vol-size":"1048576"}}`))
		}
	})

	out, err := a.ResizeDisk(context.Background(), "d1", "2g")
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, "/api/json/v2/types/volumes", putPath)
	assert.Equal(t, []string{"d1"}, putQ["name"], "a name is a parameter, not a path element")
	assert.Equal(t, "2048M", gotPut["vol-size"], "the array is told a size in megabytes")
	assert.Equal(t, "xt1", gotPut["cluster-id"], "every write names the cluster")
}

// TestAResizeByIncrementAddsToWhatTheVolumeHas covers the "+1g" form, where
// the new size is read from the array rather than given.
func TestAResizeByIncrementAddsToWhatTheVolumeHas(t *testing.T) {
	var gotPut map[string]any
	a, _ := newTestArray(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&gotPut)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		// The array reports a size in kilobytes: 1 GiB.
		_, _ = w.Write([]byte(`{"content":{"name":"d1","naa-name":"naa.1","index":7,"vol-size":"1048576"}}`))
	})

	_, err := a.ResizeDisk(context.Background(), "d1", "+1g")
	require.NoError(t, err)
	assert.Equal(t, "2048M", gotPut["vol-size"], "one gibibyte added to the one it had")
}

// TestAVolumeNamedByIndexIsAPathElement is the other half of how the array
// reads a volume name.
func TestAVolumeNamedByIndexIsAPathElement(t *testing.T) {
	var path string
	a, _ := newTestArray(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"content":{"name":"d1","naa-name":"naa.1","index":7,"vol-size":"1048576"}}`))
	})
	_, err := a.GetVolume(context.Background(), "7")
	require.NoError(t, err)
	assert.Equal(t, "/api/json/v2/types/volumes/7", path)
}

// TestAReadIsScopedToTheCluster pins that a read names the cluster it is
// about, as v2 names it: one endpoint may serve several.
func TestAReadIsScopedToTheCluster(t *testing.T) {
	var query map[string][]string
	a, _ := newTestArray(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`{"content":{"name":"d1","naa-name":"naa.1","index":7,"vol-size":"1048576"}}`))
	})
	_, err := a.GetVolume(context.Background(), "d1")
	require.NoError(t, err)
	assert.Equal(t, []string{"xt1"}, query["cluster-name"])
	assert.Equal(t, []string{"1"}, query["full"])
}

// TestTheArrayRefusalIsReported pins that an error the array answers with is
// not read as a volume.
func TestTheArrayRefusalIsReported(t *testing.T) {
	a, _ := newTestArray(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"vol_obj_name_too_long"}`))
	})
	_, err := a.GetVolume(context.Background(), "d1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vol_obj_name_too_long")
}
