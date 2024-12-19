package daemonapi

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/core/client"
)

func (a *DaemonAPI) GetNodeSSHKeys(ctx echo.Context, nodename string) error {
	if a.localhost == nodename {
		return a.getLocalSSHKeys(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSSHKeys(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalSSHKeys(ctx echo.Context) error {
	log := LogHandler(ctx, "GetNodeSSHKeys")
	b := bytes.NewBuffer(nil)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(homeDir + "/.ssh/id_*.pub")
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Warnf("failed to open file %s: %s", file, err)
			continue
		}

		_, _, _, _, err = ssh.ParseAuthorizedKey(bytes.TrimSpace(data))
		if err != nil {
			log.Warnf("skipping invalid key in file %s: %s", file, err)
			continue
		}
		b.Write(data)
	}
	return ctx.String(http.StatusOK, b.String())
}
