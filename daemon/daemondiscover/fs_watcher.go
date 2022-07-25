package daemondiscover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

func (d *discover) fsWatcherStart() (func(), error) {
	log := d.log.With().Str("func", "fsWatch").Logger()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("start")
		return func() {}, err
	}
	cleanup := func() {
		if err = watcher.Close(); err != nil {
			log.Error().Err(err).Msg("close")
		}
	}
	d.fsWatcher = watcher
	for _, filename := range []string{rawconfig.Paths.Etc} {
		if err := d.fsWatcher.Add(filename); err != nil {
			log.Error().Err(err).Msgf("add %s", filename)
			cleanup()
			return func() {}, err
		}
	}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(d.ctx)
	stop := func() {
		log.Debug().Msg("stop")
		cancel()
		wg.Wait()
	}
	wg.Add(1)
	started := make(chan bool)
	go func() {
		defer wg.Done()
		defer cleanup()
		started <- true
		log.Info().Msg("started")
		err = filepath.Walk(
			rawconfig.Paths.Etc,
			func(filename string, info os.FileInfo, err error) error {
				if info.IsDir() {
					if strings.HasPrefix(filename, rawconfig.Paths.Etc+"/namespaces/") {
						if err := d.fsWatcher.Add(filename); err != nil {
							log.Error().Err(err).Msgf("add %s", filename)
						}
					}
					return nil
				}
				nodeConf := filepath.Join(rawconfig.Paths.Etc, "node.conf")
				if filename == nodeConf {
					return nil
				}
				if strings.HasSuffix(filename, ".conf") {
					// TODO need detect node.conf to call rawconfig.LoadSections()
					p, err := filenameToPath(filename)
					if err != nil {
						log.Error().Err(err).Msgf("%s", filename)
						return nil
					}
					log.Info().Msgf("found file %s %s", filename, p)
					d.cfgCmdC <- moncmd.New(moncmd.CfgFsWatcherCreate{Path: p, Filename: filename})
				}
				return nil
			},
		)
		if err != nil {
			log.Error().Err(err).Msg("walk")
		}
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("stopped")
				return
			case e := <-watcher.Errors:
				log.Error().Err(e).Msg("")
			case event := <-watcher.Events:
				var p path.T
				filename := event.Name
				if !strings.HasSuffix(filename, ".conf") {
					continue
				}
				nodeConf := filepath.Join(rawconfig.Paths.Etc, "node.conf")
				if filename == nodeConf {
					continue
				}
				log.Debug().Msgf("event: %s", event)
				createDeleteMask := fsnotify.Create | fsnotify.Remove | fsnotify.Create
				if event.Op&createDeleteMask == 0 {
					continue
				}
				p, err := filenameToPath(filename)
				if err != nil {
					log.Warn().Err(err).Msgf("%s", filename)
				}
				switch {
				case event.Op&fsnotify.Create != 0:
					log.Debug().Msgf("detect created file %s", filename)
					d.cfgCmdC <- moncmd.New(moncmd.CfgFsWatcherCreate{Path: p, Filename: event.Name})
				}
			}
		}
	}()
	<-started
	return stop, nil
}

func filenameToPath(filename string) (path.T, error) {
	svcName := strings.TrimPrefix(filename, rawconfig.Paths.Etc+"/")
	svcName = strings.TrimPrefix(svcName, "namespaces/")
	svcName = strings.TrimSuffix(svcName, ".conf")
	if len(svcName) == 0 {
		return path.T{}, errors.New("skipped null filename")
	}
	return path.Parse(svcName)
}
