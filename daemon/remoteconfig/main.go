// Package remoteconfig defines functions to fetch object config file from api
//
// TODO move daemon/remoteconfig to core/remoteconfig since it is not anymore dedicated to daemon ?
package remoteconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/path"
)

func FetchObjectFile(cli *client.T, p path.T) (filename string, updated time.Time, err error) {
	var (
		b       []byte
		tmpFile *os.File
	)
	b, updated, err = fetchFromApi(cli, p)
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

func fetchFromApi(cli *client.T, p path.T) (b []byte, updated time.Time, err error) {
	var (
		readB []byte
	)
	handle := cli.NewGetObjectConfigFile()
	handle.ObjectSelector = p.String()
	if readB, err = handle.Do(); err != nil {
		return
	}
	type response struct {
		Data    []byte
		Updated time.Time `json:"mtime"`
	}
	resp := response{}
	if err = json.Unmarshal(readB, &resp); err != nil {
		return
	}
	return resp.Data, resp.Updated, nil
}
