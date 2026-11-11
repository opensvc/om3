package daemonapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerpodman"
)

// The keyword definitions say of a keyword whose value names resources of
// the object the groups and the shape of what it names, for a designer to
// offer the sections of the configuration, and say nothing of a keyword
// naming none. The cli documentation rendered from them says it too.
func TestGetObjectConfigKeywordsReferences(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	ctx.Set("grants", rbac.Grants{rbac.GrantRoot})
	ctx.Set("logger", plog.NewDefaultLogger())
	a := &DaemonAPI{localhost: "node1"}
	drv := "container.podman"
	require.NoError(t, a.GetObjectConfigKeywords(ctx, "test", naming.KindSvc, "web", api.GetObjectConfigKeywordsParams{Driver: &drv}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var r api.KeywordDefinitionList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &r))

	items := make(map[string]api.KeywordDefinitionItem)
	for _, item := range r.Items {
		items[item.Option] = item
	}
	netns, ok := items["netns"]
	require.True(t, ok)
	require.NotNil(t, netns.References)
	assert.Equal(t, []string{"container"}, netns.References.Groups)
	assert.Equal(t, api.KeywordReferencesShape("rid"), netns.References.Shape)
	assert.Equal(t, []string{"", "none", "host"}, netns.Candidates)

	userns, ok := items["userns"]
	require.True(t, ok)
	require.NotNil(t, userns.References)
	assert.Empty(t, userns.Candidates, "podman takes userns modes of its own")

	requires, ok := items["start_requires"]
	require.True(t, ok)
	require.NotNil(t, requires.References)
	assert.Equal(t, []string{}, requires.References.Groups)
	assert.Equal(t, api.KeywordReferencesShape("requires"), requires.References.Shape)

	image, ok := items["image"]
	require.True(t, ok)
	assert.Nil(t, image.References)
	assert.NotContains(t, rec.Body.String(), `"references":null`)

	var buf strings.Builder
	require.NoError(t, commoncmd.Doc(&buf, r.Items, naming.KindSvc, drv, "netns", 0))
	assert.Regexp(t, `references:\s+container rid`, buf.String())
}
