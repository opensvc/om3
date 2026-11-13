package commoncmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/daemon/api"
)

func TestObjectComplianceModulesetAttachment(t *testing.T) {
	var got *http.Request
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.WriteHeader(http.StatusNoContent)
	})
	_, err := ObjectComplianceModulesetAttachment("ms1", true)(context.Background(), c, testPath)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/object/path/ns1/svc/s1/compliance/moduleset/attachment", got.URL.Path)
	assert.Equal(t, "ms1", got.URL.Query().Get("moduleset"))

	_, err = ObjectComplianceRulesetAttachment("r1", false)(context.Background(), c, testPath)
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.Method)
	assert.Equal(t, "/api/object/path/ns1/svc/s1/compliance/ruleset/attachment", got.URL.Path)
	assert.Equal(t, "r1", got.URL.Query().Get("ruleset"))
}

func TestObjectComplianceAttachmentProblem(t *testing.T) {
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(api.Problem{Title: "Missing grant", Detail: "admin:ns1", Status: 403})
	})
	_, err := ObjectComplianceModulesetAttachment("ms1", true)(context.Background(), c, testPath)
	require.Error(t, err)
	assert.Equal(t, "ns1/svc/s1: [403] Missing grant: admin:ns1", err.Error())
}

func TestObjectCollectorTag(t *testing.T) {
	var (
		got  *http.Request
		body string
	)
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(api.CollectorTagAttachments{{TagName: "t1", TagData: "d1"}})
		default:
			_ = json.NewEncoder(w).Encode(api.CollectorInfo{Info: "done"})
		}
	})
	names, err := ObjectCollectorTagShow(false)(context.Background(), c, testPath)
	require.NoError(t, err)
	assert.Equal(t, "/api/object/path/ns1/svc/s1/collector/tag/attachment", got.URL.Path)
	assert.Equal(t, []string{"t1"}, names)
	l, err := ObjectCollectorTagShow(true)(context.Background(), c, testPath)
	require.NoError(t, err)
	assert.Equal(t, collector.TagAttachmentList{{TagName: "t1", TagData: "d1"}}, l)

	data := "x"
	info, err := ObjectCollectorTagAttach("t1", &data)(context.Background(), c, testPath)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/object/path/ns1/svc/s1/collector/tag/name/t1/attachment", got.URL.Path)
	assert.JSONEq(t, `{"attach_data": "x"}`, body)
	assert.Equal(t, "done", info)

	_, err = ObjectCollectorTagDetach("t1")(context.Background(), c, testPath)
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.Method)

	_, err = ObjectCollectorTagDetach("")(context.Background(), c, testPath)
	require.Error(t, err, "a tag is named")
}

func TestClusterCollectorTags(t *testing.T) {
	var (
		got  *http.Request
		body string
	)
	c := apiServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(api.CollectorTagNames{"t1", "t2"})
		default:
			_ = json.NewEncoder(w).Encode(api.CollectorInfo{Info: "created"})
		}
	})
	names, err := collectorTagNames(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "/api/cluster/collector/tag", got.URL.Path)
	assert.Equal(t, []string{"t1", "t2"}, names)

	exclude := "t2"
	info, err := collectorTagCreate(context.Background(), c, "t3", nil, &exclude)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.JSONEq(t, `{"name": "t3", "exclude": "t2"}`, body)
	assert.Equal(t, "created", info)
}
