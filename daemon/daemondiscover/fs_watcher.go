package daemondiscover

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

var (
	pathEtc = rawconfig.Paths.Etc
)

func (d *discover) fsWatcherStart() error {
	watcher, err := fsnotify.NewWatcher()
	log := d.log.With().Str("_func", "fsWatcherStart").Logger()
	if err != nil {
		d.log.Error().Err(err).Msg("NewWatcher")
		return err
	}
	cleanup := func() {
		if err = watcher.Close(); err != nil {
			log.Error().Err(err).Msg("watcher close")
		}
	}
	d.fsWatcher = watcher
	//pathEtc := rawconfig.Paths.Etc
	for _, filename := range []string{pathEtc} {
		if err := d.fsWatcher.Add(filename); err != nil {
			log.Error().Err(err).Msgf("watcher.Add %s", filename)
			cleanup()
			return err
		}
	}
	go func() {
		defer cleanup()
		log.Info().Msg("fsWatcher started")
		err = filepath.Walk(
			pathEtc,
			func(filename string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}
				if strings.HasSuffix(filename, ".conf") {
					// TODO need detect node.conf to call rawconfig.LoadSections()
					p, err := filenameToPath(filename)
					if err != nil {
						log.Error().Err(err).Msgf("fsWatcher %s", filename)
						return nil
					}
					log.Info().Msgf("cfg discover found file %s %s", filename, p)
					d.cfgCmdC <- moncmd.New(moncmd.CfgFsWatcherCreate{Path: p, Filename: filename})
				}
				return nil
			},
		)
		if err != nil {
			log.Error().Err(err).Msg("fsWatcher walk")
		}
		for {
			select {
			case <-d.ctx.Done():
				log.Info().Msg("fsWatcher done")
				return
			case e := <-watcher.Errors:
				log.Error().Err(e).Msg("fsWatcher")
			case event := <-watcher.Events:
				var p path.T
				filename := event.Name
				if !strings.HasSuffix(filename, ".conf") {
					continue
				}
				log.Debug().Msgf("event: %s", event)
				createDeleteMask := fsnotify.Create | fsnotify.Remove | fsnotify.Create
				if event.Op&createDeleteMask == 0 {
					continue
				}
				p, err := filenameToPath(filename)
				if err != nil {
					log.Warn().Err(err).Msgf("fsWatcher %s", filename)
				}
				switch {
				case event.Op&fsnotify.Create != 0:
					log.Debug().Msgf("cfg discover detect created file %s", filename)
					d.cfgCmdC <- moncmd.New(moncmd.CfgFsWatcherCreate{Path: p, Filename: event.Name})
				}
			}
		}
	}()
	return nil
}

func filenameToPath(filename string) (path.T, error) {
	svcName := strings.TrimPrefix(filename, pathEtc+"/")
	svcName = strings.TrimSuffix(svcName, ".conf")
	if len(svcName) == 0 {
		return path.T{}, errors.New("skipped null filename")
	}
	return path.Parse(svcName)
}
