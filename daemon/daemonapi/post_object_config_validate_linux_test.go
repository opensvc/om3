//go:build linux

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

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/plog"
)

// configRequest returns the context of a request of the configuration body
// by a caller holding grants, and the recorder of its answer.
func configRequest(method, target, body string, grants rbac.Grants) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "application/octet-stream")
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", grants)
	return c, rec
}

// validateDraft validates the configuration body of test/svc/foo as a caller
// holding grants, and returns the alerts answered.
func validateDraft(t *testing.T, body string, grants rbac.Grants) api.ConfigAlertItems {
	t.Helper()
	c, rec := configRequest(http.MethodPost, "/api/object/path/test/svc/foo/config/validate", body, grants)
	require.NoError(t, (&DaemonAPI{localhost: "dev1n1"}).PostObjectConfigValidate(c, "test", naming.KindSvc, "foo"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l api.ConfigAlertList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	assert.Equal(t, "ConfigAlertList", string(l.Kind))
	require.NotNil(t, l.Items, "an empty list, not null")
	return l.Items
}

func alertOfKind(l api.ConfigAlertItems, kind string) (api.ConfigAlert, bool) {
	for _, e := range l {
		if e.Kind == kind {
			return e, true
		}
	}
	return api.ConfigAlert{}, false
}

// The warnings of the validation reach the author of a draft, which a write
// does not answer: an address both named and drawn from a network.
func TestValidateADraftSaysTheWarnings(t *testing.T) {
	knownNodes(t)
	require.NoError(t, capabilities.Scan(t.Context()), "a section of a driver this node can not use is not validated further")
	l := validateDraft(t, `
[ip#1]
type = host
name = 1.2.3.4
network = default
`, rbac.Grants{rbac.GrantRoot})
	e, ok := alertOfKind(l, "conflicting strategies")
	require.True(t, ok, "%+v", l)
	assert.Equal(t, api.ConfigAlertLevelWarning, e.Level)
	assert.Equal(t, "dev1n1", e.Node)
	assert.Equal(t, "test/svc/foo", e.Path)
}

// A keyword the grants of the caller do not allow is an error alert of kind
// denied, naming the keyword, and its reason, and the validation goes on.
func TestValidateADraftSaysTheKeywordsDenied(t *testing.T) {
	knownNodes(t)
	l := validateDraft(t, `
[container#1]
type = oci
image = alpine
privileged = true
`, admin)
	e, ok := alertOfKind(l, api.ConfigAlertKindDenied)
	require.True(t, ok, "%+v", l)
	assert.Equal(t, api.ConfigAlertLevelError, e.Level)
	assert.Equal(t, "container#1.privileged", e.Key)
	assert.Contains(t, e.Comment, "requires the root grant")
	assert.NotContains(t, e.Comment, "denied: ", "the kind says it")
}

// A draft of an object that does not exist is validated as its create,
// writing nothing: no configuration, and no directory of the object.
func TestValidateADraftWritesNothing(t *testing.T) {
	knownNodes(t)
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "foo"}
	l := validateDraft(t, `
[DEFAULT]
nodes = *

[env]
foo = bar
`, admin)
	assert.Empty(t, l)
	assert.False(t, file.Exists(p.ConfigFile()), "no configuration written")
	assert.False(t, file.Exists(p.VarDir()), "no directory of the object made")
}

// A configuration a write would land is answered an empty list.
func TestValidateACleanDraftAnswersNoAlert(t *testing.T) {
	knownNodes(t)
	assert.Empty(t, validateDraft(t, `
[DEFAULT]
nodes = *
`, admin))
}

// A body that does not parse is a bad request, as the write answers it.
func TestValidateADraftThatDoesNotParse(t *testing.T) {
	knownNodes(t)
	c, rec := configRequest(http.MethodPost, "/api/object/path/test/svc/foo/config/validate", "[DEFAULT\nnodes = *\n", admin)
	require.NoError(t, (&DaemonAPI{localhost: "dev1n1"}).PostObjectConfigValidate(c, "test", naming.KindSvc, "foo"))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// A caller who may not write in the namespace may not validate there either.
func TestValidateADraftIsTheWritersOfTheNamespace(t *testing.T) {
	knownNodes(t)
	c, rec := configRequest(http.MethodPost, "/api/object/path/other/svc/foo/config/validate", "", admin)
	require.NoError(t, (&DaemonAPI{localhost: "dev1n1"}).PostObjectConfigValidate(c, "other", naming.KindSvc, "foo"))
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// A dry run of a create answers the alerts its checks found, the refusals of
// the write as errors, and writes nothing.
func TestADryRunCreateAnswersTheAlerts(t *testing.T) {
	knownNodes(t)
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "foo"}
	c, rec := configRequest(http.MethodPost, "/api/object/path/test/svc/foo/config/file?dry_run=true", `
[container#1]
type = oci
image = alpine
privileged = true
`, admin)
	dryRun := true
	require.NoError(t, (&DaemonAPI{localhost: "dev1n1"}).PostObjectConfigFile(c, "test", naming.KindSvc, "foo", api.PostObjectConfigFileParams{DryRun: &dryRun}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l api.ConfigAlertList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	e, ok := alertOfKind(l.Items, api.ConfigAlertKindDenied)
	require.True(t, ok, "%+v", l.Items)
	assert.Equal(t, "container#1.privileged", e.Key)
	assert.False(t, file.Exists(p.ConfigFile()), "no configuration written")
	assert.False(t, file.Exists(p.VarDir()), "no directory of the object made")
}

// The write itself is still refused at the first problem, as it was.
func TestACreateIsStillRefusedForADeniedKeyword(t *testing.T) {
	knownNodes(t)
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "foo"}
	c, rec := configRequest(http.MethodPost, "/api/object/path/test/svc/foo/config/file", `
[container#1]
type = oci
image = alpine
privileged = true
`, admin)
	require.NoError(t, (&DaemonAPI{localhost: "dev1n1"}).PostObjectConfigFile(c, "test", naming.KindSvc, "foo", api.PostObjectConfigFileParams{}))
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "privileged")
	assert.False(t, file.Exists(p.ConfigFile()), "no configuration written")
}
