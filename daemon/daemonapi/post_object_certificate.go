package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/key"
)

// PostObjectCertificateCreate creates the certificate of a sec, as the
// certificate create command does.
func (a *DaemonAPI) PostObjectCertificateCreate(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, store, done, err := a.certificateKeyStore(ctx, namespace, kind, name, false)
	if done {
		return err
	}
	if store == nil {
		return a.proxyCertificate(ctx, p, false, func(c *client.T) (int, []byte, error) {
			resp, err := c.PostObjectCertificateCreateWithResponse(ctx.Request().Context(), namespace, kind, name)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		})
	}
	if err := store.GenCert(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Create certificate", "%s: %s", p, err)
	}
	a.announceConfigFileWritten(p)
	return ctx.NoContent(http.StatusNoContent)
}

// PostObjectCertificateRenew renews the certificate of a sec when it is due,
// as the certificate renew command does.
func (a *DaemonAPI) PostObjectCertificateRenew(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectCertificateRenewParams) error {
	p, store, done, err := a.certificateKeyStore(ctx, namespace, kind, name, true)
	if done {
		return err
	}
	if store == nil {
		return a.proxyCertificate(ctx, p, true, func(c *client.T) (int, []byte, error) {
			resp, err := c.PostObjectCertificateRenewWithResponse(ctx.Request().Context(), namespace, kind, name, &params)
			if err != nil {
				return 0, nil, err
			}
			return resp.StatusCode(), resp.Body, nil
		})
	}
	opts := object.CertificateRenewOptions{}
	if params.Force != nil {
		opts.Force = *params.Force
	}
	result, err := store.RenewCertificate(ctx.Request().Context(), opts)
	if result.Renewed {
		a.announceConfigFileWritten(p)
	}
	switch {
	case errors.Is(err, object.ErrCertificateNotIssued):
		return JSONProblemf(ctx, http.StatusConflict, "Renew certificate", "%s: %s", p, err)
	case err != nil:
		return JSONProblemf(ctx, http.StatusInternalServerError, "Renew certificate", "%s: %s", p, err)
	}
	resp := api.CertificateRenewal{
		Domains: result.Domains,
		Reason:  result.Reason,
		Renewed: result.Renewed,
	}
	if result.Directory != "" {
		resp.Directory = &result.Directory
	}
	if !result.NotAfter.IsZero() {
		resp.NotAfter = &result.NotAfter
	}
	if resp.Domains == nil {
		resp.Domains = []string{}
	}
	return ctx.JSON(http.StatusOK, resp)
}

// certificateKeyStore returns the path and, on a node holding its
// configuration, the key store whose certificate the request issues. done
// says the request is answered, with err.
//
// A certificate is a credential, so issuing one asks more than writing keys:
//   - the requester is admin of the namespace of the ca signing it, as the
//     certificate of a cluster ca authenticates its holder as its common name,
//     which the sec chooses;
//   - the requester is root to renew from an ACME directory proving the
//     domains from acme.webroot, a host directory the daemon writes in.
func (a *DaemonAPI) certificateKeyStore(ctx echo.Context, namespace string, kind naming.Kind, name string, renew bool) (naming.Path, object.KeyStore, bool, error) {
	if v, err := assertAdmin(ctx, namespace); !v {
		return naming.Path{}, nil, true, err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return p, nil, true, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	instanceConfigData := instance.ConfigData.GetByPath(p)
	if len(instanceConfigData) == 0 {
		return p, nil, true, JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
	}
	if _, ok := instanceConfigData[a.localhost]; !ok {
		return p, nil, false, nil
	}
	if v, err := assertUsrKeyWrite(ctx, p); !v {
		return p, nil, true, err
	}
	o, err := object.New(p)
	if err != nil {
		return p, nil, true, JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s: %s", p, err)
	}
	store, ok := o.(object.KeyStore)
	if !ok {
		return p, nil, true, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a keystore", p)
	}
	cfg := o.(object.Configurer).Config()
	acme := renew && cfg.GetString(key.Parse("acme.directory")) != ""
	if s := cfg.GetString(key.Parse("ca")); s != "" && !acme {
		caPath, err := naming.ParsePath(s)
		if err != nil {
			return p, nil, true, JSONProblemf(ctx, http.StatusBadRequest, "Invalid ca", "%s: %s", p, err)
		}
		if caPath.Namespace != p.Namespace {
			if v, err := assertAdmin(ctx, caPath.Namespace); !v {
				return p, nil, true, err
			}
		}
	}
	if acme && cfg.GetString(key.Parse("acme.webroot")) != "" {
		if v, err := assertRoot(ctx); !v {
			return p, nil, true, err
		}
	}
	return p, store, false, nil
}

// proxyCertificate passes the request to the first node holding the
// configuration of p, which answers for it. A renewal waits for the ACME
// directory, longer than a client allows an answer by default.
func (a *DaemonAPI) proxyCertificate(ctx echo.Context, p naming.Path, long bool, fn func(*client.T) (int, []byte, error)) error {
	for nodename := range instance.ConfigData.GetByPath(p) {
		var opts []funcopt.O
		if long {
			opts = append(opts, client.WithTimeout(0))
		}
		c, err := a.newProxyClient(ctx, nodename, opts...)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		status, body, err := fn(c)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		}
		if len(body) > 0 {
			return ctx.JSONBlob(status, body)
		}
		return ctx.NoContent(status)
	}
	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
}
