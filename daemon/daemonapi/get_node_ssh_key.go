package daemonapi

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/util/command"
)

func (a *DaemonAPI) GetNodeSSHKey(ctx echo.Context, nodename string) error {
	if a.localhost == nodename {
		return a.getLocalSSHKey(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSSHKey(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalSSHKey(ctx echo.Context) error {
	nodeConfig := node.ConfigData.GetByNode(a.localhost)
	log := LogHandler(ctx, "GetNodeSSHKeys")
	b := bytes.NewBuffer(nil)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keyFile := filepath.Join(homeDir, ".ssh", nodeConfig.SSHKey)
	pubFile := filepath.Join(homeDir, ".ssh", nodeConfig.SSHKey+".pub")
	data, err := os.ReadFile(pubFile)
	if errors.Is(err, os.ErrNotExist) {
		err = command.New(
			command.WithName("ssh-keygen"),
			command.WithVarArgs("-N", "", "-q", "-f", keyFile),
			command.WithLogger(log),
			command.WithCommandLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.ErrorLevel),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
		).Run()
		if err != nil {
			log.Warnf("%s", err)
			return err
		}
		data, err = os.ReadFile(pubFile)
	}
	if err != nil {
		log.Warnf("%s", err)
		return err
	}

	_, _, _, _, err = ssh.ParseAuthorizedKey(bytes.TrimSpace(data))
	if err != nil {
		log.Warnf("invalid pubkey in file %s: %s", pubFile, err)
		return err
	}
	b.Write(data)
	return ctx.String(http.StatusOK, b.String())
}
