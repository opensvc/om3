package daemonapi

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonAPI) GetNodeSSHKey(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
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
		args := []string{
			"-N", "", "-q", "-f", keyFile,
			"-C", fmt.Sprintf("opensvc@%s sshkey=%s %s", a.localhost, nodeConfig.SSHKey, time.Now().Format(time.RFC3339)),
		}
		err = command.New(
			command.WithName("ssh-keygen"),
			command.WithArgs(args),
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
		if err != nil {
			log.Warnf("%s", err)
			return err
		}
		for _, name := range []string{keyFile, pubFile} {
			// sync file to prevent unrecoverable empty file on crash
			if err := file.Sync(name); err != nil {
				log.Warnf("%s", err)
				return err
			}
		}
	} else if err != nil {
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
