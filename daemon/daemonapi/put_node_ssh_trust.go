package daemonapi

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/util/sshnode"
)

func (a *DaemonAPI) PutNodeSSHTrust(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename == a.localhost {
		return a.localPutNodeSSHTrust(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PutNodeSSHTrust(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) localPutNodeSSHTrust(ctx echo.Context) error {
	log := LogHandler(ctx, "PutNodeSSHTrust")

	clusterConfigData := cluster.ConfigData.Get()
	authorizedKeys, err := sshnode.GetAuthorizedKeysMap()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "parse authorized_keys", "%s", err)
	}

	doNodeAuthorizedKeys := func(node string, c *client.T) error {
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
			key, _, _, _, err := ssh.ParseAuthorizedKey(line)
			if err != nil {
				return fmt.Errorf("failed to parse node %s key: %w", node, err)
			}
			fingerprint := ssh.FingerprintLegacyMD5(key)
			if v, _ := authorizedKeys.Has(line); v {
				log.Infof("node %s is already trusted by key %s", node, fingerprint)
			} else if err := sshnode.AppendAuthorizedKeys(line); err != nil {
				return fmt.Errorf("node %s key couldn't be added to the authorized_keys file: %s", node, err)
			} else {
				log.Infof("node %s key added to the authorized_keys file: %s", node, fingerprint)
			}
		}
		// Check for errors
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	}
	doNodeKnownHosts := func(node string, c *client.T) error {
		resp, err := c.GetNodeSSHHostkeys(ctx.Request().Context(), node)
		if err != nil {
			return err
		}
		switch resp.StatusCode {
		case http.StatusOK:
		default:
			return fmt.Errorf("get ssh host keys from %s: %s", node, resp.Status)
		}
		defer resp.Body.Close()

		knownHosts, err := sshnode.GetKnownHostsMap()
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			key, _, _, _, err := ssh.ParseAuthorizedKey(line)
			if err != nil {
				return fmt.Errorf("failed to parse node %s host key: %w", node, err)
			}
			fingerprint := ssh.FingerprintLegacyMD5(key)
			if err := knownHosts.Add(node, key); err != nil {
				return fmt.Errorf("node %s key couldn't be added to the known_hosts file: %s", node, err)
			} else {
				log.Infof("node %s key added to the known_hosts file: %s", node, fingerprint)
			}
		}
		// Check for errors
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	}
	doNode := func(node string) error {
		c, err := a.newProxyClient(ctx, node)
		if err != nil {
			return err
		}
		if err := doNodeAuthorizedKeys(node, c); err != nil {
			return err
		}
		if err := doNodeKnownHosts(node, c); err != nil {
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
