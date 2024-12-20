package daemonapi

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/sshnode"
)

func (a *DaemonAPI) PutNodeSSHTrust(ctx echo.Context, nodename string) error {
	if nodename == a.localhost {
		return a.localPutNodeSSHTrust(ctx, nodename)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PutNodeSSHTrust(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) localPutNodeSSHTrust(ctx echo.Context, nodename string) error {
	log := LogHandler(ctx, "PutNodeSSHTrust")
	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}

	clusterConfigData := cluster.ConfigData.Get()
	authorizedKeys, err := sshnode.GetAuthorizedKeysMap()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "parse authorized_keys", "%s", err)
	}

	doNode := func(node string) error {
		c, err := a.newProxyClient(ctx, node)
		if err != nil {
			return err
		}
		resp, err := c.GetNodeSSHKey(ctx.Request().Context(), node)
		if err != nil {
			return err
		}
		switch resp.StatusCode {
		case http.StatusOK:
		default:
			return fmt.Errorf("get ssh key from %s: %s", node, resp.Status)
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Bytes()
			if v, _ := authorizedKeys.Has(line); v {
				log.Infof("node %s is already trusted by key %s", node, string(line))
				continue
			}
			if err := sshnode.AppendAuthorizedKeys(line); err != nil {
				return err
			} else {
				log.Infof("trust node %s by key %s", node, string(line))
			}
		}

		// Check for errors
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil

	}

	var errs error
	for _, node := range clusterConfigData.Nodes {
		if node == a.localhost {
			continue
		}
		if err := doNode(node); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "trust nodes", "%s", errs)
	}
	return ctx.NoContent(http.StatusNoContent)
}
