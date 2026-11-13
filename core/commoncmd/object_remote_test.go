package commoncmd

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/compliance"
)

var testPath = naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}

// The compliance run of an object is asked of the instance endpoint of the
// node, with the options of the run.
func TestObjectComplianceRun(t *testing.T) {
	var got *http.Request
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiComplianceRun())
	})
	run, err := objectComplianceRun(context.Background(), c, testPath, "node1", compliance.ActionCheck, ComplianceRunOptions{Modules: "mod1", Attach: true})
	require.NoError(t, err)
	assert.Equal(t, "/api/node/name/node1/instance/path/ns1/svc/s1/compliance/action/check", got.URL.Path)
	assert.Equal(t, "mod1", got.URL.Query().Get("module"))
	assert.Equal(t, "true", got.URL.Query().Get("attach"))
	require.Len(t, run.ModuleActions, 2)
}

func TestObjectComplianceRunProblem(t *testing.T) {
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(api.Problem{Title: "Missing grant", Detail: "admin:ns1", Status: 403})
	})
	_, err := objectComplianceRun(context.Background(), c, testPath, "node1", compliance.ActionFix, ComplianceRunOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ns1/svc/s1")
	assert.Contains(t, err.Error(), "node node1")
	assert.Contains(t, err.Error(), "admin:ns1")
}

func TestObjectKeyInstall(t *testing.T) {
	var got *http.Request
	accepted := api.InstanceActionAccepted{SessionID: uuid.New(), ExecID: uuid.New()}
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(accepted)
	})
	p := naming.Path{Namespace: "ns1", Kind: naming.KindSec, Name: "k1"}
	data, err := objectKeyInstall(context.Background(), c, p, "node1", "tls.crt")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/node/name/node1/instance/path/ns1/sec/k1/action/key/install", got.URL.Path)
	assert.Equal(t, "tls.crt", got.URL.Query().Get("name"))
	assert.NotEmpty(t, got.URL.Query().Get("session_id"), "the exec joins the session of the requester")
	assert.Equal(t, accepted, data)

	_, err = objectKeyInstall(context.Background(), c, p, "node1", "")
	require.NoError(t, err)
	assert.False(t, got.URL.Query().Has("name"), "no name installs all the keys")
}

// A node with no instance of the object has nothing to list, which is no
// error of the listing.
func TestInstanceListingsOfANodeWithoutInstance(t *testing.T) {
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(api.Problem{Title: "No local instance", Status: 404})
	})
	devices, err := instanceDevices(context.Background(), c, testPath, "node1", "all")
	require.NoError(t, err)
	assert.Empty(t, devices)
	infos, err := instanceResourceInfo(context.Background(), c, testPath, "node1")
	require.NoError(t, err)
	assert.Empty(t, infos)
}

func TestInstanceDevices(t *testing.T) {
	var got *http.Request
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.InstanceDeviceList{
			Kind: api.InstanceDeviceListKindInstanceDeviceList,
			Items: api.InstanceDeviceItems{{
				Kind: "InstanceDeviceItem",
				Meta: api.InstanceMeta{Node: "node1", Object: "ns1/svc/s1"},
				Data: api.InstanceDevice{Path: "ns1/svc/s1", Rid: "disk#1", Driver: api.InstanceDeviceDriver{Group: "disk", Name: "disk"}, Role: api.InstanceDeviceRoleBase, Device: "/dev/sda"},
			}},
		})
	})
	l, err := instanceDevices(context.Background(), c, testPath, "node1", "base")
	require.NoError(t, err)
	assert.Equal(t, "/api/node/name/node1/instance/path/ns1/svc/s1/device", got.URL.Path)
	assert.Equal(t, "base", got.URL.Query().Get("roles"))
	require.Len(t, l, 1)
	assert.Equal(t, "node1", l[0].Node)
	assert.Equal(t, "/dev/sda", l[0].Device)
}

func TestFilterResourceInfoItems(t *testing.T) {
	items := api.ResourceInfoItems{{Rid: "fs#1", Key: "a"}, {Rid: "fs#2", Key: "b"}, {Rid: "ip#1", Key: "c"}}
	assert.Equal(t, items, filterResourceInfoItems(items, ""))
	assert.Equal(t, api.ResourceInfoItems{{Rid: "fs#1", Key: "a"}, {Rid: "ip#1", Key: "c"}}, filterResourceInfoItems(items, "fs#1,ip#1"))
}
