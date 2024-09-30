// Package remoteconfig defines functions to fetch object config file from api
//
// TODO move daemon/remoteconfig to core/remoteconfig since it is not anymore dedicated to daemon ?
package remoteconfig

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func FetchObjectConfigFile(cli *client.T, p naming.Path) (filename string, updated time.Time, err error) {
	var (
		b       []byte
		tmpFile *os.File
	)
	b, updated, err = fetchFromAPI(cli, p)
	if err != nil {
		return
	}
	dstFile := p.ConfigFile()
	dstDir := filepath.Dir(dstFile)

	tmpFile, err = os.CreateTemp(dstDir, p.Name+".conf.*.tmp")
	if errors.Is(err, os.ErrNotExist) {
		if err = os.MkdirAll(dstDir, os.ModePerm); err != nil {
			return
		}
		if tmpFile, err = os.CreateTemp(dstDir, p.Name+".conf.*.tmp"); err != nil {
			return
		}
	} else if err != nil {
		return
	}
	defer func() {
		_ = tmpFile.Close()
	}()
	filename = tmpFile.Name()
	if _, err = tmpFile.Write(b); err != nil {
		return
	}
	if err = os.Chtimes(filename, updated, updated); err != nil {
		return
	}
	return
}

func fetchFromAPI(cli *client.T, p naming.Path) (b []byte, updated time.Time, err error) {
	var (
		mtime time.Time
		resp  *api.GetInstanceConfigFileResponse
	)
	resp, err = cli.GetInstanceConfigFileWithResponse(context.Background(), cli.Hostname(), p.Namespace, p.Kind, p.Name)
	if err != nil {
		return
	} else if resp.StatusCode() != http.StatusOK {
		err = fmt.Errorf("unexpected get object file %s status %s", p, resp.Status())
		return
	}
	if mtime, err = time.Parse(time.RFC3339Nano, resp.HTTPResponse.Header.Get(api.HeaderLastModifiedNano)); err != nil {
		return
	}
	return resp.Body, mtime, nil
}
