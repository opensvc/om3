package remoteconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/timestamp"
)

func Fetch(ctx context.Context, p path.T, node string, cmdC chan<- *moncmd.T) {
	id := daemondata.InstanceId(p, node)
	log := daemonlogctx.Logger(ctx).With().Str("_pkg", "remoteconfig").Str("id", id).Logger()
	b, updated, err := fetchFromApi(p, node)
	if err != nil {
		log.Error().Err(err).Msgf("fetchFromApi %s", id)
		return
	}
	f, err := os.CreateTemp("", p.Name+".conf.*.tmp")
	if err != nil {
		log.Error().Err(err).Msgf("CreateTemp for %s", id)
		return
	}
	tmpFilename := f.Name()
	defer func() {
		log.Debug().Msgf("done fetcher routine for %s@%s", p, node)
		_ = os.Remove(tmpFilename)
	}()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		log.Error().Err(err).Msgf("write tmp file for %s", id)
		return
	}
	_ = f.Close()
	mtime := updated.Time()
	if err := os.Chtimes(tmpFilename, mtime, mtime); err != nil {
		log.Error().Err(err).Msgf("update file time %s", tmpFilename)
		return
	}
	configure, err := object.NewConfigurer(p, object.WithConfigFile(f.Name()), object.WithVolatile(true))
	if err != nil {
		log.Error().Err(err).Msgf("configure error for %s", p)
		return
	}
	nodes := configure.Config().Referrer.Nodes()
	validScope := false
	for _, n := range nodes {
		if n == hostname.Hostname() {
			validScope = true
			break
		}
	}
	if !validScope {
		log.Info().Msgf("invalid scope %s", nodes)
		return
	}
	select {
	case <-ctx.Done():
		log.Info().Msgf("abort fetch config %s", id)
		return
	default:
		err := make(chan error)
		cmdC <- moncmd.New(moncmd.RemoteFileConfig{
			Path:     p,
			Node:     node,
			Filename: f.Name(),
			Updated:  updated,
			Ctx:      ctx,
			Err:      err,
		})
		<-err
	}
}

func fetchFromApi(p path.T, node string) (b []byte, updated timestamp.T, err error) {
	url := fmt.Sprintf("raw://%s:%d", node, daemonenv.RawPort)
	var (
		cli   *client.T
		readB []byte
	)
	if cli, err = client.New(client.WithURL(url)); err != nil {
		return
	}
	handle := cli.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	if readB, err = handle.Do(); err != nil {
		return
	}
	type response struct {
		Data    string
		Updated timestamp.T `json:"mtime"`
	}
	resp := response{}
	if err = json.Unmarshal(readB, &resp); err != nil {
		return
	}
	return []byte(resp.Data), resp.Updated, nil
}
