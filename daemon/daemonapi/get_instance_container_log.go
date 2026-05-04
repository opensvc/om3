package daemonapi

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNodeInstanceContainerLog serves container logs
func (a *DaemonAPI) GetInstanceContainerLog(ctx echo.Context, nodename string, namespace string, kind naming.Kind, name string, params api.GetInstanceContainerLogParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	nodename = a.parseNodename(nodename)

	if nodename == a.localhost || nodename == "localhost" {
		return a.getLocalNodeInstanceContainerLog(ctx, namespace, kind, name, params)
	} else if clusternode.Has(nodename) {
		return a.getPeerInstanceContainerLog(ctx, nodename, namespace, kind, name, params)
	} else {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
}

func (a *DaemonAPI) getLocalNodeInstanceContainerLog(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetInstanceContainerLogParams) error {
	log := LogHandler(ctx, "GetNodeInstanceContainerLog")
	log.Tracef("starting")
	defer log.Tracef("done")

	// Set up response for streaming
	w := ctx.Response()
	request := ctx.Request()
	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}
	w.WriteHeader(http.StatusOK)
	w.Flush()

	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		JSONProblemf(ctx, http.StatusBadRequest, "Command setup failed", "%s", err)
	}

	// Execute the om command to get container logs
	cmdArgs := []string{path.String(), "container", "logs"}
	if params.Rid != nil {
		cmdArgs = append(cmdArgs, "--rid", *params.Rid)
	}
	if params.Follow != nil && *params.Follow {
		cmdArgs = append(cmdArgs, "--follow")
	}
	if params.Lines != nil {
		cmdArgs = append(cmdArgs, "--lines", fmt.Sprint(*params.Lines))
	}

	cmd := exec.CommandContext(ctx.Request().Context(), "om", cmdArgs...)
	cmd.Dir = "/"

	log.Tracef("prepared command: %s", cmd)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Command setup failed", "failed to get stdout pipe: %s", err)
	}

	// Get stderr pipe
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Command setup failed", "failed to get stderr pipe: %s", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Command failed", "failed to start om command: %s", err)
	}

	// Stream stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Warnf("Error reading stdout: %s", err)
				break
			}
			if n > 0 {
				if _, err := w.Write(buf[:n]); err != nil {
					log.Warnf("Error writing to response: %s", err)
					break
				}
				w.Flush()
			}
		}
	}()

	// Stream stderr to logs
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Warnf("Error reading stderr: %s", err)
				break
			}
			if n > 0 {
				log.Warnf("om command stderr: %s", strings.TrimSpace(string(buf[:n])))
			}
		}
	}()

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		log.Warnf("om command finished with error: %s", err)
	}

	return nil
}

func (a *DaemonAPI) getPeerInstanceContainerLog(ctx echo.Context, nodename string, namespace string, kind naming.Kind, name string, params api.GetInstanceContainerLogParams) error {
	log := LogHandler(ctx, "GetNodeInstanceContainerLog")

	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		JSONProblemf(ctx, http.StatusBadRequest, "Command setup failed", "%s", err)
	}

	evCtx := ctx.Request().Context()
	request := ctx.Request()

	c, err := a.newProxyClient(ctx, nodename, client.WithTimeout(0))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	w := ctx.Response()

	rid := ""
	if params.Rid != nil {
		rid = *params.Rid
	}
	containerLogs := c.NewGetContainerLogs(path, nodename, rid)

	// Get the logs stream with parameters from the request
	follow := false
	lines := 100
	if params.Follow != nil {
		follow = *params.Follow
	}
	if params.Lines != nil {
		lines = int(*params.Lines)
	}

	logChan, err := containerLogs.Logs(evCtx, follow, lines)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	}

	// Set response headers for streaming
	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	// Stream the log data
	for logData := range logChan {
		if _, err := w.Write(logData); err != nil {
			log.Tracef("error writing to response: %s", err)
			break
		}
		w.Flush()
	}

	return nil
}
