package discover

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/pubsub"
)

const (
	delayExistAfterRemove = 100 * time.Millisecond
)

func dirCreated(event fsnotify.Event) bool {
	if event.Op&fsnotify.Create == 0 {
		return false
	}
	if stat, err := os.Stat(event.Name); os.IsNotExist(err) {
		return false
	} else if err != nil {
		log.Error().Err(err).Msgf("stat %s", event.Name)
		return false
	} else if !stat.IsDir() {
		return false
	}
	return true
}

func dirRemoved(event fsnotify.Event) bool {
	if event.Op&fsnotify.Remove == 0 {
		return false
	}
	if stat, err := os.Stat(event.Name); err != nil {
		return false
	} else if !stat.IsDir() {
		return false
	}
	return true
}

func (d *discover) fsWatcherStart() (func(), error) {
	log := d.log.With().Str("func", "fsWatch").Logger()
	bus := pubsub.BusFromContext(d.ctx)
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
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(d.ctx)
	stop := func() {
		log.Info().Msg("stopping")
		cancel()
		wg.Wait()
	}
	nodeConf := rawconfig.NodeConfigFile()

	//
	// Add directory watch for head and its subdirs, and for .conf files
	//
	initDirWatches := func(head string) error {
		return filepath.WalkDir(
			head,
			func(filename string, entry fs.DirEntry, err error) error {
				switch {
				case entry.IsDir():
					if err := d.fsWatcher.Add(filename); err != nil {
						log.Error().Err(err).Msgf("add dir watch %s", filename)
					} else {
						log.Info().Msgf("add dir watch %s", filename)
					}
				default:
					if !strings.HasSuffix(filename, ".conf") {
						return nil
					}
					var (
						p   path.T
						err error
					)
					if filename == nodeConf {
						// pass
					} else if p, err = cfgFilenameToPath(filename); err != nil {
						log.Warn().Err(err).Msgf("do not watch invalid config file %s", filename)
						return nil
					}
					/*
						if err := watcher.Add(filename); err != nil {
							log.Error().Err(err).Msgf("add file %s", filename)
						} else {
							log.Debug().Msgf("add file %s", filename)
						}
					*/
					bus.Pub(&msgbus.ConfigFileUpdated{Path: p, File: filename}, pubsub.Label{"path", p.String()})
				}
				return nil
			},
		)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cleanup()
		log.Info().Msg("started")
		defer log.Info().Msg("stopped")
		const createDeleteMask = fsnotify.Create | fsnotify.Remove
		const needReAddMask = fsnotify.Remove | fsnotify.Rename
		const updateMask = fsnotify.Remove | fsnotify.Rename | fsnotify.Write | fsnotify.Create | fsnotify.Chmod

		// Add directory watches for:
		//  etc/
		//  var/node/
		varNodeDir := filepath.Join(rawconfig.Paths.Var, "node")
		nodeFrozenFile := filepath.Join(varNodeDir, "frozen")
		for _, dir := range []string{rawconfig.Paths.Etc, varNodeDir} {
			if err := d.fsWatcher.Add(dir); err != nil {
				log.Error().Err(err).Msgf("add dir watch %s", dir)
			} else {
				log.Info().Msgf("add dir watch %s", dir)
			}
		}

		if updated := file.ModTime(nodeFrozenFile); !updated.IsZero() {
			log.Info().Msgf("detect %s initially exists", nodeFrozenFile)
			bus.Pub(&msgbus.NodeFrozenFileUpdated{File: nodeFrozenFile, At: updated}, pubsub.Label{"node", d.localhost})
		} else {
			log.Info().Msgf("detect %s initially absent", nodeFrozenFile)
			bus.Pub(&msgbus.NodeFrozenFileRemoved{File: nodeFrozenFile}, pubsub.Label{"node", d.localhost})
		}

		if err := initDirWatches(rawconfig.Paths.Etc); err != nil {
			log.Error().Err(err).Msgf("init fs watches walking %s", rawconfig.Paths.Etc)
		}

		// watcher-events handler loop
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-watcher.Errors:
				log.Error().Err(e).Send()
			case event := <-watcher.Events:
				log.Debug().Msgf("event: %s", event)
				filename := event.Name
				switch {
				case strings.HasSuffix(filename, "frozen"):
					if filename != nodeFrozenFile {
						// TODO: track instance frozen flag ? the namespace var is not yet watched
						continue
					}
					switch {
					case event.Op&fsnotify.Remove != 0:
						log.Debug().Msgf("detect removed file %s (%s)", filename, event.Op)
						if filename == nodeFrozenFile {
							bus.Pub(&msgbus.NodeFrozenFileRemoved{File: filename}, pubsub.Label{"node", d.localhost})
						}
					case event.Op&updateMask != 0:
						if event.Op&needReAddMask != 0 {
							time.Sleep(delayExistAfterRemove)
							if !file.Exists(filename) {
								log.Info().Msg("file removed")
								continue
							} else {
								if err := watcher.Add(filename); err != nil {
									log.Error().Err(err).Msgf("re-add file watch %s", filename)
								} else {
									log.Debug().Msgf("re-add file watch %s", filename)
								}
							}
						}
						log.Debug().Msgf("detect updated file %s (%s)", filename, event.Op)
						if filename == nodeFrozenFile {
							bus.Pub(&msgbus.NodeFrozenFileUpdated{File: filename, At: file.ModTime(filename)}, pubsub.Label{"node", d.localhost})
						}
					}
				case strings.HasSuffix(filename, ".conf"):
					var (
						p   path.T
						err error
					)
					if filename == nodeConf {
						rawconfig.LoadSections()
					} else if p, err = cfgFilenameToPath(filename); err != nil {
						log.Warn().Err(err).Msgf("%s", filename)
						continue
					}
					switch {
					case event.Op&fsnotify.Remove != 0:
						log.Debug().Msgf("detect removed file %s (%s)", filename, event.Op)
						bus.Pub(&msgbus.ConfigFileRemoved{Path: p, File: filename}, pubsub.Label{"path", p.String()})
					case event.Op&updateMask != 0:
						if event.Op&needReAddMask != 0 {
							time.Sleep(delayExistAfterRemove)
							if !file.Exists(filename) {
								log.Info().Msg("file removed")
								continue
							} else {
								if err := watcher.Add(filename); err != nil {
									log.Error().Err(err).Msgf("re-add file watch %s", filename)
								} else {
									log.Debug().Msgf("re-add file watch %s", filename)
								}
							}
						}
						log.Debug().Msgf("detect updated file %s (%s)", filename, event.Op)
						bus.Pub(&msgbus.ConfigFileUpdated{Path: p, File: filename}, pubsub.Label{"path", p.String()})
					}
				case dirCreated(event):
					if event.Name == "." {
						log.Debug().Msgf("skip event %s", event)
						continue
					}
					if err := d.fsWatcher.Add(filename); err != nil {
						log.Error().Err(err).Msgf("add dir watch %s", filename)
					} else {
						log.Info().Msgf("add dir watch %s", filename)
					}
					if err := initDirWatches(filename); err != nil {
						log.Error().Err(err).Msgf("init fs watches walking %s", filename)
					}
				case dirRemoved(event):
					if err := d.fsWatcher.Remove(filename); err != nil {
						log.Error().Err(err).Msgf("remove dir watch %s", filename)
					} else {
						log.Info().Msgf("remove dir watch %s", filename)
					}
				}
			}
		}
	}()
	return stop, nil
}

func cfgFilenameToPath(filename string) (path.T, error) {
	return filenameToPath(filename, rawconfig.Paths.Etc, ".conf")
}

func filenameToPath(filename, prefix, suffix string) (path.T, error) {
	svcName := strings.TrimPrefix(filename, prefix+"/")
	svcName = strings.TrimPrefix(svcName, "namespaces/")
	svcName = strings.TrimSuffix(svcName, suffix)
	if len(svcName) == 0 {
		return path.T{}, fmt.Errorf("skipped null filename")
	}
	return path.Parse(svcName)
}
