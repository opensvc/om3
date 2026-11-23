package tmplrender

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netstatus"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/label"
)

// fakeFetcher serves the keys of keys, by "path key", and the keywords of
// config, by "path kw node", and counts the fetches.
type fakeFetcher struct {
	keys    map[string]string
	config  map[string]string
	fetches int
}

func (f *fakeFetcher) DataKey(_ context.Context, p naming.Path, name string) ([]byte, error) {
	f.fetches++
	v, ok := f.keys[p.String()+" "+name]
	if !ok {
		return nil, ErrNotFound
	}
	return []byte(v), nil
}

func (f *fakeFetcher) DataKeyNames(_ context.Context, p naming.Path) ([]string, error) {
	f.fetches++
	var l []string
	for k := range f.keys {
		if path, name, _ := strings.Cut(k, " "); path == p.String() {
			l = append(l, name)
		}
	}
	if l == nil {
		return nil, ErrNotFound
	}
	return l, nil
}

func (f *fakeFetcher) ConfigValue(_ context.Context, p naming.Path, kw, node string) (string, error) {
	f.fetches++
	v, ok := f.config[p.String()+" "+kw+" "+node]
	if !ok {
		return "", fmt.Errorf("keyword %s: %w", kw, ErrNotFound)
	}
	return v, nil
}

func (f *fakeFetcher) ResourceInfo(_ context.Context, p naming.Path, node, rid, key string) (string, error) {
	f.fetches++
	return p.String() + " " + node + " " + rid + " " + key, nil
}

func actorStatus(avail status.T) object.Status {
	return object.Status{ActorStatus: &object.ActorStatus{Avail: avail, Overall: avail}}
}

// newData returns the cluster data of two nodes, n1 serving it, with a web
// svc up on both, its ip and disk resources, a db svc down, and a cfg.
func newData() *clusterdump.Data {
	data := clusterdump.NewData("n1")
	data.Cluster.Object["prod/svc/web"] = actorStatus(status.Up)
	data.Cluster.Object["prod/svc/db"] = actorStatus(status.Down)
	data.Cluster.Object["prod/cfg/db"] = object.Status{}
	for i, name := range []string{"n1", "n2"} {
		data.Cluster.Node[name] = node.Node{
			Config: node.Config{Env: "PRD", Labels: label.M{"rack": fmt.Sprintf("r%d", i+1)}},
			Status: node.Status{IsLeader: name == "n1"},
			Instance: map[string]instance.Instance{
				"prod/svc/web": {
					Config:  &instance.Config{Checksum: "w1", Scope: []string{"n1", "n2"}, ActorConfig: &instance.ActorConfig{App: "shop"}},
					Monitor: &instance.Monitor{State: instance.MonitorStateIdle},
					Status: &instance.Status{
						Avail: status.Up,
						Resources: instance.ResourceStatuses{
							"ip#1":   {Type: "ip.host", Status: status.Up, Info: map[string]any{"ipaddr": fmt.Sprintf("10.0.0.%d", i+1), "dev": "eth0"}},
							"disk#1": {Type: "disk.lvm", Status: status.Up},
							"app#1":  {Type: "app.forking", Status: status.Up},
						},
					},
				},
				"prod/cfg/db": {Config: &instance.Config{Checksum: "c1"}},
				"prod/sec/db": {Config: &instance.Config{Checksum: "s1"}},
				"prod/cfg/f":  {Config: &instance.Config{Checksum: "f1"}},
			},
			Network: map[string]netstatus.Status{"backend": {Type: "routed_bridge", Network: "10.1.0.0/16"}},
		}
	}
	return data
}

func newRenderer(text string, f Fetcher) *Renderer {
	return &Renderer{Name: "test", Text: text, Namespace: "prod", Fetcher: f, Data: map[string]any{"env": "prod"}}
}

func doRender(t *testing.T, r *Renderer, st *State) (string, Result) {
	t.Helper()
	res, err := r.Render(context.Background(), st)
	require.NoError(t, err)
	return string(res.Out), res
}

func TestKey(t *testing.T) {
	f := &fakeFetcher{keys: map[string]string{"prod/cfg/db host": "db1", "prod/sec/db password": "s3cr3t"}}
	st := NewState(newData())
	out, res := doRender(t, newRenderer(`{{ key "./cfg/db" "host" }} {{ key "prod/sec/db" "password" }}`, f), st)
	assert.Equal(t, "db1 s3cr3t", out)
	assert.True(t, res.ReadsSecret)
	assertSubscription(t, dataStoreKinds, "prod/cfg/db,prod/sec/db", SubscriptionOf(res.Reads))
	assert.Empty(t, Changed(res.Reads, st))

	// A change of the configuration of the cfg on a node is a new version
	// of its keys.
	n := st.Data.Cluster.Node["n2"]
	n.Instance["prod/cfg/db"] = instance.Instance{Config: &instance.Config{Checksum: "c2"}}
	assert.Equal(t, []string{"key prod/cfg/db host"}, Changed(res.Reads, st))
}

func TestKeyMissing(t *testing.T) {
	f := &fakeFetcher{keys: map[string]string{"prod/cfg/db host": "db1"}}
	st := NewState(newData())

	out, _ := doRender(t, newRenderer(`{{ keyOr "./cfg/db" "port" "5432" }} {{ keyExists "./cfg/db" "port" }} {{ keyExists "./cfg/db" "host" }}`, f), st)
	assert.Equal(t, "5432 false true", out)

	// A strict read failing records the reads made, so the datastore is
	// watched, and the template renders again when the key appears.
	res, err := newRenderer(`{{ key "./cfg/db" "port" }}`, f).Render(context.Background(), st)
	require.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, res.Out)
	require.Len(t, res.Reads, 1)
	assert.Equal(t, "prod/cfg/db", res.Reads[0].Selection)
}

func TestKeyRejectsOtherKinds(t *testing.T) {
	_, err := newRenderer(`{{ key "usr/admin" "password" }}`, &fakeFetcher{}).Render(context.Background(), NewState(newData()))
	require.ErrorContains(t, err, "is not a cfg or sec datastore")
}

func TestKeyNames(t *testing.T) {
	f := &fakeFetcher{keys: map[string]string{"prod/cfg/f flag_b": "1", "prod/cfg/f flag_a": "0", "prod/cfg/f other": "x"}}
	out, _ := doRender(t, newRenderer(`{{ range keyNames "./cfg/f" "flag_*" }}{{ . }}={{ key "./cfg/f" . }} {{ end }}{{ len (keyNames "./cfg/none") }}`, f), NewState(newData()))
	assert.Equal(t, "flag_a=0 flag_b=1 0", out)
}

func TestFetchOncePerRender(t *testing.T) {
	f := &fakeFetcher{keys: map[string]string{"prod/cfg/db host": "db1"}}
	doRender(t, newRenderer(`{{ key "./cfg/db" "host" }}{{ key "./cfg/db" "host" }}{{ key "prod/cfg/db" "host" }}`, f), NewState(newData()))
	assert.Equal(t, 1, f.fetches)
}

func TestComputedPath(t *testing.T) {
	f := &fakeFetcher{keys: map[string]string{"prod/cfg/db host": "db1"}}
	_, res := doRender(t, newRenderer(`{{ key (printf "%s/cfg/db" .env) "host" }}`, f), NewState(newData()))
	assert.Equal(t, "key prod/cfg/db host", res.Reads[0].ID)
}

func TestAvail(t *testing.T) {
	st := NewState(newData())
	r := newRenderer(`{{ avail "./svc/web" }} {{ availOr "./svc/gone" "down" }} {{ objectExists "./svc/gone" }}`, nil)
	out, res := doRender(t, r, st)
	assert.Equal(t, "up down false", out)

	// A refresh of the status with no change of what was read changes no
	// fingerprint.
	o := st.Data.Cluster.Object["prod/svc/web"]
	o.UpdatedAt = time.Now()
	st.Data.Cluster.Object["prod/svc/web"] = o
	assert.Empty(t, Changed(res.Reads, st))

	st.Data.Cluster.Object["prod/svc/web"] = actorStatus(status.Down)
	assert.Equal(t, []string{"avail prod/svc/web"}, Changed(res.Reads, st))

	st.Data.Cluster.Object["prod/svc/gone"] = actorStatus(status.Up)
	assert.Equal(t, []string{"avail prod/svc/web", "availOr prod/svc/gone down", "objectExists prod/svc/gone"}, Changed(res.Reads, st))
}

func TestAvailOfAMissingObjectIsAnError(t *testing.T) {
	_, err := newRenderer(`{{ avail "./svc/gone" }}`, nil).Render(context.Background(), NewState(newData()))
	require.ErrorIs(t, err, ErrNotFound)
}

func TestObjects(t *testing.T) {
	st := NewState(newData())
	out, res := doRender(t, newRenderer(`{{ range objects "prod/svc/*" }}{{ .name }}:{{ .avail }} {{ end }}`, nil), st)
	assert.Equal(t, "db:down web:up ", out)
	assertSubscription(t, objectKinds, "prod/svc/*", SubscriptionOf(res.Reads))

	// An object leaving the range is a change, not an error.
	delete(st.Data.Cluster.Object, "prod/svc/db")
	assert.Equal(t, []string{"objects prod/svc/*"}, Changed(res.Reads, st))
	out, _ = doRender(t, newRenderer(`{{ range objects "prod/svc/*" }}{{ .name }}:{{ .avail }} {{ end }}`, nil), st)
	assert.Equal(t, "web:up ", out)
}

func TestObjectsSelectorUnion(t *testing.T) {
	out, _ := doRender(t, newRenderer(`{{ range objects "prod/svc/web,prod/cfg/*" }}{{ .path }} {{ end }}`, nil), NewState(newData()))
	assert.Equal(t, "prod/cfg/db prod/svc/web ", out)
}

func TestObjectsRefusesConfigSelectors(t *testing.T) {
	_, err := newRenderer(`{{ objects "app=shop" }}`, nil).Render(context.Background(), NewState(newData()))
	require.ErrorContains(t, err, "can't use filter")
}

func TestRangeWithNoSelectorFollowsAllObjects(t *testing.T) {
	_, res := doRender(t, newRenderer(`{{ range objects }}{{ end }}{{ avail "./svc/web" }}`, nil), NewState(newData()))
	assertSubscription(t, objectKinds, "", SubscriptionOf(res.Reads))
}

func TestInstances(t *testing.T) {
	st := NewState(newData())
	out, _ := doRender(t, newRenderer(`{{ range instances "prod/svc/*" }}{{ .path }}@{{ .node }}:{{ .avail }}:{{ .app }}:{{ .monitor.state }} {{ end }}`, nil), st)
	assert.Equal(t, "prod/svc/web@n1:up:shop:idle prod/svc/web@n2:up:shop:idle ", out)

	out, _ = doRender(t, newRenderer(`{{ (instanceStatus "./svc/web" "n2").avail }}`, nil), st)
	assert.Equal(t, "up", out)

	_, err := newRenderer(`{{ instanceStatus "./svc/web" "n3" }}`, nil).Render(context.Background(), st)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestResources(t *testing.T) {
	st := NewState(newData())
	out, _ := doRender(t, newRenderer(`{{ range ips "prod/svc/web" }}{{ .node }}:{{ .rid }}:{{ .ipaddr }}:{{ .dev }} {{ end }}`, nil), st)
	assert.Equal(t, "n1:ip#1:10.0.0.1:eth0 n2:ip#1:10.0.0.2:eth0 ", out)

	out, _ = doRender(t, newRenderer(`{{ range disks }}{{ .rid }} {{ end }}|{{ range apps }}{{ .rid }} {{ end }}|{{ range resources "prod/svc/web" "ip#*" }}{{ .rid }} {{ end }}`, nil), st)
	assert.Equal(t, "disk#1 disk#1 |app#1 app#1 |ip#1 ip#1 ", out)
}

func TestNodes(t *testing.T) {
	st := NewState(newData())
	out, res := doRender(t, newRenderer(`{{ range nodes "rack=r2" }}{{ .name }}{{ end }} {{ range nodes "n*" }}{{ .name }}:{{ .is_leader }} {{ end }}{{ nodeLabel "n1" "rack" }}`, nil), st)
	assert.Equal(t, "n2 n1:true n2:false r1", out)
	assertSubscription(t, nodeKinds, "", SubscriptionOf(res.Reads))

	n := st.Data.Cluster.Node["n1"]
	n.Config.Labels["rack"] = "r9"
	assert.Equal(t, []string{"nodes n*", "nodeLabel n1 rack"}, Changed(res.Reads, st))
}

func TestNetworks(t *testing.T) {
	out, _ := doRender(t, newRenderer(`{{ range networks "back*" }}{{ .name }}@{{ .node }}:{{ .network }} {{ end }}`, nil), NewState(newData()))
	assert.Equal(t, "backend@n1:10.1.0.0/16 backend@n2:10.1.0.0/16 ", out)
}

func TestConfig(t *testing.T) {
	f := &fakeFetcher{config: map[string]string{"prod/svc/web app#1.port ": "8080", "prod/svc/web app#1.port n2": "8081"}}
	st := NewState(newData())
	out, res := doRender(t, newRenderer(`{{ config "./svc/web" "app#1.port" }} {{ configAs "./svc/web" "app#1.port" "n2" }}`, f), st)
	assert.Equal(t, "8080 8081", out)

	// The value as n2 evaluates it depends on the configuration of n2, not
	// of n1, the node serving the api.
	n := st.Data.Cluster.Node["n2"]
	n.Config.Env = "DEV"
	st.Data.Cluster.Node["n2"] = n
	assert.Equal(t, []string{"config prod/svc/web app#1.port n2"}, Changed(res.Reads, st))
}

func TestResinfo(t *testing.T) {
	st := NewState(newData())
	out, res := doRender(t, newRenderer(`{{ resinfo "./svc/web" "ip#1" "ipaddr" }}`, &fakeFetcher{}), st)
	assert.Equal(t, "prod/svc/web n1 ip#1 ipaddr", out)
	st.ResourceInfo[ResourceInfoID("prod/svc/web", "n1")] = "new"
	assert.Len(t, Changed(res.Reads, st), 1)
}

func TestReadsRecordedOnAFailedRender(t *testing.T) {
	res, err := newRenderer(`{{ avail "./svc/web" }}{{ avail "./svc/gone" }}{{ avail "./svc/db" }}`, nil).Render(context.Background(), NewState(newData()))
	require.Error(t, err)
	ids := make([]string, len(res.Reads))
	for i, r := range res.Reads {
		ids[i] = r.ID
	}
	assert.Equal(t, []string{"avail prod/svc/web", "avail prod/svc/gone"}, ids)
}

func TestParse(t *testing.T) {
	require.NoError(t, newRenderer(`{{ range objects }}{{ upper .name }}{{ end }}`, nil).Parse())
	require.ErrorContains(t, newRenderer(`{{ exec "ls" }}`, nil).Parse(), `function "exec" not defined`)
	require.Error(t, newRenderer(`{{ if }}`, nil).Parse())
}

func TestMissingDataKeyIsAnError(t *testing.T) {
	_, err := newRenderer(`{{ .nope }}`, nil).Render(context.Background(), NewState(newData()))
	require.ErrorContains(t, err, "nope")
}

func assertSubscription(t *testing.T, kinds []string, selector string, got Subscription) {
	t.Helper()
	assert.ElementsMatch(t, kinds, got.Kinds)
	assert.Equal(t, selector, got.Selector)
}

// The api answers an empty value for a key of a datastore no node has: the
// cluster data tells it does not exist.
func TestKeyOfAMissingDataStore(t *testing.T) {
	f := &fakeFetcher{}
	st := NewState(newData())
	_, err := newRenderer(`{{ key "prod/cfg/none" "host" }}`, f).Render(context.Background(), st)
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorContains(t, err, "datastore prod/cfg/none")

	out, res := doRender(t, newRenderer(`{{ keyOr "prod/cfg/none" "host" "dflt" }} {{ keyExists "prod/cfg/none" "host" }} {{ len (keyNames "prod/cfg/none") }}`, f), st)
	assert.Equal(t, "dflt false 0", out)
	assert.Equal(t, 0, f.fetches, "no api call for a datastore no node has")

	// The datastore created, the reads change.
	n := st.Data.Cluster.Node["n1"]
	n.Instance["prod/cfg/none"] = instance.Instance{Config: &instance.Config{Checksum: "x"}}
	assert.Len(t, Changed(res.Reads, st), 2)
}
