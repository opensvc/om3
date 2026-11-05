package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// A node configuration the validation refuses is answered with the keywords
// it refuses, not with the error of a validation that ran.
func TestWriteNodeConfigFileSaysWhatIsWrong(t *testing.T) {
	testhelper.Setup(t)
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("[node]\noci = nosuchengine\n"))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	require.NoError(t, (&DaemonAPI{}).writeNodeConfigFile(c, hostname.Hostname()))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotContains(t, rec.Body.String(), "%!s(<nil>)")
	assert.Contains(t, rec.Body.String(), "node.oci")
}
