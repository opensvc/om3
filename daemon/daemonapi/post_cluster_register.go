package daemonapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/credential"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/command"
)

// PostClusterRegister registers every cluster node on the collector, by
// forking a 'om cluster register' in the background.
//
// The command posts to the register endpoint of every cluster node, so each
// node registers itself: the collector mints a registration id for a
// nodename, and a node cannot register on behalf of another.
//
// The processing is asynchronous, as the registration of a node covers the
// initial asset, package and disk push it sends, and the command does that
// for every node in turn.
func (a *DaemonAPI) PostClusterRegister(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	log := LogHandler(ctx, "PostClusterRegister")

	var payload api.ClusterRegisterBody
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	cred := ""
	if payload.Credential != nil && *payload.Credential != "" {
		// Parsed here as well as by the forked command, so a malformed
		// credential is answered to the caller instead of being found in a
		// log after the fork.
		if _, _, err := credential.Parse(*payload.Credential); err != nil {
			log.Infof("register refused: %s", err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "field 'credential': %s", err)
		}
		cred = *payload.Credential
	}

	execname, err := os.Executable()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "can't detect om execname: %s", err)
	}
	cmd := command.New(
		command.WithName(execname),
		command.WithArgs(clusterRegisterArgs(payload)),
		command.WithEnv(clusterRegisterEnviron(os.Environ(), cred)),
	)

	if err := cmd.Start(); err != nil {
		log.Errorf("start cluster register: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "cluster register failed: %s", err)
	}
	log.Infof("forked a background cluster register")
	return JSONProblemf(ctx, http.StatusOK, "background cluster register has been called", "registering the cluster nodes on the collector")
}

// clusterRegisterArgs returns the arguments of the cluster register to fork.
//
// The credential is not among them: a command line is readable by any user
// through /proc/<pid>/cmdline.
func clusterRegisterArgs(payload api.ClusterRegisterBody) []string {
	args := []string{"cluster", "register"}
	if payload.App != nil && *payload.App != "" {
		args = append(args, "--app", *payload.App)
	}
	return args
}

// clusterRegisterEnviron returns environ with the collector credential added,
// which is how the forked command is handed it: /proc/<pid>/environ is not
// world readable, unlike /proc/<pid>/cmdline.
func clusterRegisterEnviron(environ []string, cred string) []string {
	if cred == "" {
		return environ
	}
	return append(environ, env.CollectorCredentialVar+"="+cred)
}
