package daemonapi

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// configReadAccess asserts the caller may read the configuration of p, and
// says whether the secrets it holds must be redacted from what is read.
//
// The cluster configuration holds the cluster secret, which every sec and usr
// value is encrypted with, so it is read by root alone, and by a node joining
// the cluster, which needs the secret to join.
//
// Any other configuration is read by a guest of its namespace, but its secrets
// are not: the values of the keywords declared secret, and the encrypted keys
// of the sec and usr objects, are shown to the administrators of the namespace
// and to root only. An encrypted key is a secret too, since the key it is
// encrypted with is one leak away. The administrators get them because they
// write them: an edit reads the configuration and writes it back, and must not
// write the placeholder back in place of the secret. A joining node gets them
// too: it installs the ca, the certificate and the heartbeat secrets of the
// cluster it joins, and holds the cluster secret they are encrypted with
// already.
func configReadAccess(ctx echo.Context, p naming.Path) (redact bool, ok bool, err error) {
	if p.Kind == naming.KindCcfg {
		ok, err = assertGrant(ctx, rbac.GrantRoot, rbac.GrantJoin)
		return false, ok, err
	}
	if ok, err = assertGuest(ctx, p.Namespace); !ok {
		return false, ok, err
	}
	canReadSecrets := grantsFromContext(ctx).HasGrant(
		rbac.NewGrant(rbac.RoleAdmin, p.Namespace),
		rbac.NewGrant(rbac.RoleAdmin, ""),
		rbac.GrantRoot,
		rbac.GrantJoin,
	)
	return !canReadSecrets, true, nil
}

// serveConfigFile answers the configuration file of p, with its secrets
// redacted when redact is set.
//
// A file that can not be redacted is not served: an error is better than a
// secret.
func serveConfigFile(ctx echo.Context, p naming.Path, filename string, redact bool) error {
	if !redact {
		return ctx.File(filename)
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Internal server error", "Failed to read config file: %s", filename)
	}
	kind := p.Kind.String()
	if p.Kind == naming.KindCcfg {
		kind = ""
	}
	b, err := object.RedactSecrets(content, kind)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Internal server error", "%s", fmt.Errorf("redact %s: %w", filename, err))
	}
	return ctx.Blob(http.StatusOK, "application/octet-stream", b)
}
