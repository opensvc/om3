package remoteconfig

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
)

func Fetch(ctx context.Context, p path.T, node string, cmdC chan<- *msgbus.Msg) {
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
	if err := os.Chtimes(tmpFilename, updated, updated); err != nil {
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
		cmdC <- msgbus.NewMsg(msgbus.RemoteFileConfig{
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

func fetchFromApi(p path.T, node string) (b []byte, updated time.Time, err error) {
	var (
		cli   *client.T
		readB []byte
	)
	cli, err = client.New(
		client.WithURL(daemonenv.UrlHttpNode(node)),
		client.WithPassword(rawconfig.ClusterSection().Secret),
		client.WithUsername(hostname.Hostname()),
		client.WithCertificate(daemonenv.CertFile()),
	)
	if err != nil {
		return
	}
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
