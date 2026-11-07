package daemonapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"

	_ "github.com/opensvc/om3/v3/drivers/resiphost"
)

// The keyword definitions carry the groups of the keywords they list: each
// keyword names its group, and the list describes each group once, with the
// strategies of the ip addressing. A list of no keyword has no group, written
// [] rather than null.
func TestGetObjectConfigKeywordsGroups(t *testing.T) {
	get := func(params api.GetObjectConfigKeywordsParams) (api.KeywordDefinitionList, string) {
		t.Helper()
		e := echo.New()
		rec := httptest.NewRecorder()
		ctx := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
		ctx.Set("grants", rbac.Grants{rbac.GrantRoot})
		ctx.Set("logger", plog.NewDefaultLogger())
		a := &DaemonAPI{localhost: "node1"}
		require.NoError(t, a.GetObjectConfigKeywords(ctx, "test", naming.KindSvc, "web", params))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var r api.KeywordDefinitionList
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &r))
		return r, rec.Body.String()
	}

	drv := "ip.host"
	r, _ := get(api.GetObjectConfigKeywordsParams{Driver: &drv})
	require.NotEmpty(t, r.Items)
	groups := make(map[string]api.KeywordGroupItem)
	for _, g := range r.Groups {
		groups[g.Driver+" "+g.Id] = g
	}
	for _, item := range r.Items {
		require.NotEmptyf(t, item.Group, "%s has a group", item.Option)
		_, own := groups["ip.host "+item.Group]
		_, shared := groups[" "+item.Group]
		assert.Truef(t, own || shared, "%s: group %s is described", item.Option, item.Group)
		switch item.Option {
		case "name", "netmask", "network", "addr":
			assert.Equal(t, "addressing", item.Group, item.Option)
		}
	}
	addressing, ok := groups["ip.host addressing"]
	require.True(t, ok, "the addressing group of ip.host")
	assert.Equal(t, "Addressing", addressing.Name)
	assert.NotEmpty(t, addressing.Text)
	require.Len(t, addressing.Strategies, 2)
	assert.Equal(t, "static", addressing.Strategies[0].Id)
	assert.Equal(t, []string{"name"}, addressing.Strategies[0].Uses)
	assert.Equal(t, []string{"network", "addr"}, addressing.Strategies[0].Excludes)
	assert.Equal(t, []string{}, addressing.Strategies[0].When)
	assert.Equal(t, "ipam", addressing.Strategies[1].Id)
	assert.Equal(t, []string{"network", "addr"}, addressing.Strategies[1].Uses)
	assert.Equal(t, []string{"name"}, addressing.Strategies[1].Excludes)
	_, ok = groups[" triggers"]
	assert.True(t, ok, "the triggers group every resource has")
	for i := 1; i < len(r.Groups); i++ {
		assert.LessOrEqual(t, r.Groups[i-1].Order, r.Groups[i].Order, "the groups are sorted by order")
	}

	option := "nosuchkeyword"
	r, body := get(api.GetObjectConfigKeywordsParams{Option: &option})
	assert.Empty(t, r.Items)
	assert.Contains(t, body, `"groups":[]`)
}
