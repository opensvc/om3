package daemondiscover

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemonenv"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/instcfg"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/daemon/monitor/mondata"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/timestamp"
)

func (d *discover) cfgRoutine() {
	d.log.Info().Msg("cfgRoutine started")
	defer func() {
		done := time.After(dropCmdTimeout)
		for {
			select {
			case <-done:
				return
			case <-d.cfgCmdC:
			}
		}
	}()
	c := daemonctx.DaemonPubSubCmd(d.ctx)
	defer ps.UnSub(c, ps.SubCfg(c, pubsub.OpUpdate, "discover.cfg cfg.update", "", d.onEvCfg))
	defer ps.UnSub(c, ps.SubCfg(c, pubsub.OpDelete, "discover.cfg cfg.delete", "", d.onEvCfg))

	for {
		select {
		case <-d.ctx.Done():
			d.log.Info().Msg("cfgRoutine done")
		case i := <-d.cfgCmdC:
			switch c := (*i).(type) {
			case moncmd.CfgFsWatcherCreate:
				d.cmdLocalCfgFileAdded(c.Path, c.Filename)
			case moncmd.MonCfgDone:
				d.cmdInstCfgDone(c.Path, c.Filename)
			case moncmd.CfgUpdated:
				if c.Node == d.localhost {
					continue
				}
				d.cmdRemoteCfgUpdated(c.Path, c.Node, c.Config)
			case moncmd.CfgDeleted:
				if c.Node == d.localhost {
					continue
				}
				d.cmdRemoteCfgDeleted(c.Path, c.Node)
			case moncmd.RemoteFileConfig:
				d.cmdRemoteCfgFetched(c)
			default:
				d.log.Error().Interface("cmd", i).Msg("unknown cmd")
			}
		}
	}
}

func (d *discover) onEvCfg(i interface{}) {
	d.cfgCmdC <- moncmd.New(i)
}

func (d *discover) cmdLocalCfgFileAdded(p path.T, filename string) {
	s := p.String()
	if _, ok := d.moncfg[s]; ok {
		return
	}
	instcfg.Start(d.ctx, p, filename, d.cfgCmdC)
	d.moncfg[s] = struct{}{}
}

func (d *discover) cmdInstCfgDone(p path.T, filename string) {
	s := p.String()
	if _, ok := d.moncfg[s]; ok {
		delete(d.moncfg, s)
	}
	if file.Exists(filename) {
		d.cmdLocalCfgFileAdded(p, filename)
	}
}

func (d *discover) cmdRemoteCfgUpdated(p path.T, node string, remoteCfg instance.Config) {
	s := p.String()
	d.log.Info().Msgf("cmdRemoteCfgUpdated for node %s, path %s", node, p)
	if _, ok := d.moncfg[s]; ok {
		return
	}
	if remoteUpdated, ok := d.fetcherUpdated[s]; ok {
		// fetcher in progress for s
		if remoteCfg.Updated.Time().After(remoteUpdated.Time()) {
			d.log.Info().Msgf("cancel pending remote cfg fetcher, more recent config from %s on %s", s, node)
			d.cancelFetcher(s)
		} else {
			d.log.Error().Msgf("cmdRemoteCfgUpdated for node %s, path %s not more recent", node, p)
			return
		}
	}
	if !d.inScope(&remoteCfg) {
		d.log.Error().Msgf("cmdRemoteCfgUpdated for node %s, path %s not in scope", node, p)
		return
	}
	d.log.Info().Msgf("fetch config %s from node %s", s, node)
	d.fetchCfgFromRemote(p, node, remoteCfg.Updated)
}

func (d *discover) cmdRemoteCfgDeleted(p path.T, node string) {
	s := p.String()
	if fetchFrom, ok := d.fetcherFrom[s]; ok {
		if fetchFrom == node {
			d.log.Info().Msgf("cancel pending remote cfg fetcher %s@%s not anymore present", s, node)
			d.cancelFetcher(s)
		}
	}
}

func (d *discover) cmdRemoteCfgFetched(c moncmd.RemoteFileConfig) {
	select {
	case <-c.Ctx.Done():
		c.Err <- nil
		return
	default:
		defer d.cancelFetcher(c.Path.String())
		s := c.Path.String()
		confFile := rawconfig.Node.Paths.Etc + "/" + s + ".conf"
		d.log.Info().Msgf("install fetched config %s from %s", s, c.Node)
		err := os.Rename(c.Filename, confFile)
		if err != nil {
			d.log.Error().Err(err).Msgf("can't install fetched config to %s", confFile)
		}
		c.Err <- err
	}
	return
}

func (d *discover) inScope(cfg *instance.Config) bool {
	localhost := d.localhost
	for _, node := range cfg.Scope {
		if node == localhost {
			return true
		}
	}
	return false
}

func (d *discover) cancelFetcher(s string) {
	node := d.fetcherFrom[s]
	d.fetcherCancel[s]()
	delete(d.fetcherCancel, s)
	delete(d.fetcherNodeCancel[node], s)
	delete(d.fetcherUpdated, s)
	delete(d.fetcherFrom, s)
}

func (d *discover) fetchCfgFromRemote(p path.T, node string, updated timestamp.T) {
	s := p.String()
	if n, ok := d.fetcherFrom[s]; ok {
		d.log.Error().Msgf("fetcher already in progress for %s from %s", s, n)
		return
	}
	ctx, cancel := context.WithCancel(d.ctx)
	d.fetcherCancel[s] = cancel
	d.fetcherFrom[s] = node
	d.fetcherUpdated[s] = updated
	if _, ok := d.fetcherNodeCancel[node]; ok {
		d.fetcherNodeCancel[node][s] = cancel
	} else {
		d.fetcherNodeCancel[node] = make(map[string]context.CancelFunc)
	}

	go d.fetcherRoutine(ctx, p, node)
}

func (d *discover) fetchFromApi(p path.T, node string) (b []byte, updated timestamp.T, err error) {
	url := "raw://" + node + ":" + daemonenv.RawPort
	var (
		cli   *client.T
		readB []byte
	)
	cli, err = client.New(client.WithURL(url))
	if err != nil {
		d.log.Error().Err(err).Msgf("fetchFromApi new client from %s", url)
		return
	}
	handle := cli.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	readB, err = handle.Do()
	if err != nil {
		d.log.Error().Err(err).Msg("fetchFromApi")
	}
	type response struct {
		Data    string
		Updated timestamp.T `json:"mtime"`
	}
	resp := response{}
	err = json.Unmarshal(readB, &resp)
	if err != nil {
		d.log.Error().Err(err).Msgf("fetchFromApi Unmarshal '%s'", readB)
		return
	}
	return []byte(resp.Data), resp.Updated, nil
}

func (d *discover) fetcherRoutine(ctx context.Context, p path.T, node string) {
	id := mondata.InstanceId(p, node)
	b, updated, err := d.fetchFromApi(p, node)
	if err != nil {
		d.log.Error().Err(err).Msgf("fetchFromApi %s", id)
		return
	}
	f, err := os.CreateTemp("", p.Name+".conf.*.tmp")
	if err != nil {
		d.log.Error().Err(err).Msgf("CreateTemp for %s", id)
		return
	}
	tmpFilename := f.Name()
	defer func() {
		d.log.Info().Msgf("done fetcher routine for %s@%s", p, node)
		_ = os.Remove(tmpFilename)
	}()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		d.log.Error().Err(err).Msgf("write tmp file for %s", id)
		return
	}
	_ = f.Close()
	mtime := updated.Time()
	if err := os.Chtimes(tmpFilename, mtime, mtime); err != nil {
		d.log.Error().Err(err).Msgf("update file time %s", tmpFilename)
		return
	}
	configure, err := object.NewConfigurerFromPath(p, object.WithConfigFile(f.Name()), object.WithVolatile(true))
	if err != nil {
		d.log.Error().Err(err).Msgf("configure error for %s")
		return
	}
	nodes := configure.Config().Referrer.Nodes()
	validScope := false
	for _, n := range nodes {
		if n == d.localhost {
			validScope = true
			break
		}
	}
	if !validScope {
		return
	}
	select {
	case <-ctx.Done():
		d.log.Info().Msgf("abort fetch config %s", id)
		return
	default:
		err := make(chan error)
		d.cfgCmdC <- moncmd.New(moncmd.RemoteFileConfig{
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
