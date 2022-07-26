package daemondiscover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/file"
)

const (
	delayExistAfterRemove = 100 * time.Millisecond
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
	for _, filename := range []string{rawconfig.Paths.Etc, rawconfig.Paths.Etc + "/namespaces"} {
		if err := d.fsWatcher.Add(filename); err != nil {
			log.Error().Err(err).Msgf("add %s", filename)
			cleanup()
			return func() {}, err
		} else {
			log.Info().Msgf("add dir %s", filename)
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
	go func() {
		defer wg.Done()
		defer cleanup()
		log.Info().Msg("started")
		nodeConf := filepath.Join(rawconfig.Paths.Etc, "node.conf")
		const createDeleteMask = fsnotify.Create | fsnotify.Remove
		const needReAddMask = fsnotify.Remove | fsnotify.Rename
		const updateMask = fsnotify.Remove | fsnotify.Rename | fsnotify.Write | fsnotify.Create | fsnotify.Chmod
		//
		// Add directory watch for:
		//  etc/
		//  etc/namespaces/
		//  etc/namespaces/*
		//
		// Add config file watches
		//  etc/*.conf
		//  etc/namespaces/*/*.conf
		//
		err = filepath.Walk(
			rawconfig.Paths.Etc,
			func(filename string, info os.FileInfo, err error) error {
				switch {
				case info.IsDir():
					if strings.HasPrefix(filename, rawconfig.Paths.Etc+"/namespaces/") {
						if err := d.fsWatcher.Add(filename); err != nil {
							log.Error().Err(err).Msgf("add dir watch %s", filename)
						} else {
							log.Info().Msgf("add dir watch %s", filename)
						}
					}
				case filename == nodeConf:
					// nothing special here. just watch.
					if err := watcher.Add(filename); err != nil {
						log.Error().Err(err).Msgf("add file watch %s", filename)
					} else {
						log.Debug().Msgf("add file watch %s", filename)
					}
				case strings.HasSuffix(filename, ".conf"):
					p, err := filenameToPath(filename)
					if err != nil {
						log.Warn().Err(err).Msgf("do not watch invalid config file %s", filename)
						return nil
					}
					if err := watcher.Add(filename); err != nil {
						log.Error().Err(err).Msgf("add file %s", filename)
					} else {
						log.Debug().Msgf("add file %s", filename)
					}
					d.cfgCmdC <- moncmd.New(moncmd.CfgFileUpdated{Path: p, Filename: filename})
				}
				return nil
			},
		)
		if err != nil {
			log.Error().Err(err).Msg("walk")
		}

		// watcher-events handler loop
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("stopped")
				return
			case e := <-watcher.Errors:
				log.Error().Err(e).Msg("")
			case event := <-watcher.Events:
				log.Debug().Msgf("event: %s", event)
				filename := event.Name
				switch {
				case (filename == nodeConf) && (event.Op&updateMask != 0):
					rawconfig.LoadSections()
				case strings.HasSuffix(filename, ".conf"):
					p, err := filenameToPath(filename)
					if err != nil {
						log.Warn().Err(err).Msgf("%s", filename)
					}
					switch {
					case event.Op&fsnotify.Remove != 0:
						log.Debug().Msgf("detect removed file %s", filename)
						d.cfgCmdC <- moncmd.New(moncmd.CfgFileRemoved{Path: p, Filename: filename})
					case event.Op&updateMask != 0:
						if event.Op&needReAddMask != 0 {
							time.Sleep(delayExistAfterRemove)
							if !file.Exists(filename) {
								log.Info().Msg("file removed")
								return
							} else {
								if err := watcher.Add(filename); err != nil {
									log.Error().Err(err).Msgf("re-add file watch %s", filename)
								} else {
									log.Debug().Msgf("re-add file watch %s", filename)
								}
							}
						}
						log.Debug().Msgf("detect updated file %s", filename)
						d.cfgCmdC <- moncmd.New(moncmd.CfgFileUpdated{Path: p, Filename: filename})
					}
				}

			}
		}
	}()
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
