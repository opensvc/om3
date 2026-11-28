package daemonapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

// savedStore is the image store of an engine holding one image, whose save
// writes archive then fails with saveErr.
type savedStore struct {
	pull    resource.Pull
	archive string
	saveErr error
	kindErr error
	saves   int
}

func (t *savedStore) Has(_ context.Context, pull resource.Pull) (bool, string, error) {
	if pull.Engine == "down" {
		return false, "", errors.New("the engine does not answer")
	}
	if pull == t.pull {
		return true, "sha256:abc", nil
	}
	return false, "", nil
}

func (t *savedStore) Save(ctx context.Context, pull resource.Pull, w io.Writer) (string, error) {
	t.saves++
	if present, id, err := t.Has(ctx, pull); err != nil || !present {
		return id, resource.ErrImageNotFound
	}
	_, _ = io.WriteString(w, t.archive)
	return "sha256:abc", t.saveErr
}

func (t *savedStore) IDKind(context.Context, resource.Pull) (string, error) {
	if t.kindErr != nil {
		return "", t.kindErr
	}
	return resource.ImageIDManifest, nil
}

func (t *savedStore) Load(context.Context, resource.Pull, io.Reader) error {
	return errors.New("not a load")
}

// The archive of an image is sent to root alone, with the id of the image
// and the platform of the node ahead of it. A store that does not hold the
// reference, which may be the one of a user, is a 404: the requester asks
// another node, or the registry.
func TestGetNodeImageSave(t *testing.T) {
	store := &savedStore{
		pull:    resource.Pull{Engine: "testengine", Store: "app", Image: "registry.example.com/app:1"},
		archive: "the archive",
	}
	resource.RegisterImageStore("testengine", store)
	resource.RegisterImageStore("down", store)

	get := func(t *testing.T, grants rbac.Grants, engine, userStore, ref string) *httptest.ResponseRecorder {
		t.Helper()
		q := url.Values{"engine": {engine}, "ref": {ref}}
		params := api.GetNodeImageSaveParams{Engine: engine, Ref: ref}
		if userStore != "" {
			q.Set("store", userStore)
			params.Store = &userStore
		}
		req := httptest.NewRequest(http.MethodGet, "/api/node/name/n1/image/save?"+q.Encode(), nil)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		c.Set("logger", plog.NewDefaultLogger())
		c.Set("grants", grants)
		c.Set("user", &daemonauth.Info{Username: "tester"})
		require.NoError(t, (&DaemonAPI{localhost: "n1"}).GetNodeImageSave(c, "n1", params))
		return rec
	}
	root := rbac.Grants{rbac.GrantRoot}

	t.Run("root is sent the archive", func(t *testing.T) {
		rec := get(t, root, "testengine", "app", "registry.example.com/app:1")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "the archive", rec.Body.String())
		assert.Equal(t, "sha256:abc", rec.Header().Get(api.HeaderImageID))
		assert.Equal(t, resource.ImageIDManifest, rec.Header().Get(api.HeaderImageIDKind))
		assert.Equal(t, resource.NodePlatform(), rec.Header().Get(api.HeaderPlatform))
		assert.Equal(t, echo.MIMEOctetStream, rec.Header().Get(echo.HeaderContentType))
	})

	t.Run("an engine not telling the kind of its ids still sends it", func(t *testing.T) {
		store.kindErr = errors.New("the storage driver of the engine is not known")
		defer func() { store.kindErr = nil }()
		rec := get(t, root, "testengine", "app", "registry.example.com/app:1")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "the archive", rec.Body.String())
		assert.Empty(t, rec.Header().Values(api.HeaderImageIDKind))
	})

	t.Run("no other grant is", func(t *testing.T) {
		saves := store.saves
		for name, grants := range map[string]rbac.Grants{
			"none":                     {},
			"admin of every namespace": {rbac.NewGrant(rbac.RoleAdmin, "")},
			"admin of the namespace":   {rbac.NewGrant(rbac.RoleAdmin, "ns1")},
			"guest of every namespace": {rbac.NewGrant(rbac.RoleGuest, "")},
			"heartbeat":                {rbac.GrantHeartbeat},
			"inspector":                {rbac.GrantInspector},
			"scraper":                  {rbac.GrantScraper},
		} {
			rec := get(t, grants, "testengine", "app", "registry.example.com/app:1")
			assert.Equal(t, http.StatusForbidden, rec.Code, name)
			assert.NotContains(t, rec.Body.String(), "the archive", name)
			assert.Empty(t, rec.Header().Get(api.HeaderImageID), name)
		}
		assert.Equal(t, saves, store.saves, "nothing was saved for them")
	})

	t.Run("an image the store does not hold is a 404", func(t *testing.T) {
		saves := store.saves
		for name, args := range map[string][3]string{
			"another reference":       {"testengine", "app", "registry.example.com/app:2"},
			"the store of the system": {"testengine", "", "registry.example.com/app:1"},
			"the store of another":    {"testengine", "other", "registry.example.com/app:1"},
		} {
			rec := get(t, root, args[0], args[1], args[2])
			assert.Equal(t, http.StatusNotFound, rec.Code, name)
			assert.Empty(t, rec.Header().Get(api.HeaderImageID), name)
		}
		assert.Equal(t, saves, store.saves)
	})

	t.Run("an engine the node has not is refused", func(t *testing.T) {
		rec := get(t, root, "nosuchengine", "", "registry.example.com/app:1")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		rec = get(t, root, "testengine", "app", "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("an engine that does not answer is not a missing image", func(t *testing.T) {
		rec := get(t, root, "down", "", "registry.example.com/app:1")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("a save failing once started aborts the response", func(t *testing.T) {
		store.saveErr = errors.New("no space left on device")
		defer func() { store.saveErr = nil }()
		assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
			get(t, root, "testengine", "app", "registry.example.com/app:1")
		}, "a body merely ended would read as the whole archive")
	})
}
