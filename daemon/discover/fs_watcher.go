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

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

const (
	delayExistAfterRemove = 100 * time.Millisecond
	debounceDelay         = 200 * time.Millisecond
)

type Debouncer struct {
	timer *time.Timer
}

func (d *Debouncer) Debounce(wait time.Duration, fn func()) {
	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(wait, fn)
}

func dirCreated(event fsnotify.Event) bool {
	if event.Op&fsnotify.Create == 0 {
		return false
	}
	if stat, err := os.Stat(event.Name); os.IsNotExist(err) {
		return false
	} else if err != nil {
		log.Error().Err(err).Msgf("daemon: discover: fs: stat %s", event.Name)
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

func (t *Manager) PubDebounce(bus *pubsub.Bus, key string, v pubsub.Messager, labels ...pubsub.Label) {
	debouncer, ok := t.debouncers[key]

	if !ok {
		debouncer = &Debouncer{}
		t.debouncers[key] = debouncer
	}
	debouncer.Debounce(debounceDelay, func() {
		bus.Pub(v, labels...)
	})
}

func (t *Manager) fsWatcherStart() (func(), error) {
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/discover").WithPrefix("daemon: discover: fs: ")
	bus := pubsub.BusFromContext(t.ctx)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("start: %s", err)
		return func() {}, err
	}
	cleanup := func() {
		if err = watcher.Close(); err != nil {
			log.Errorf("close: %s", err)
		}
	}
	t.fsWatcher = watcher
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.ctx)
	stop := func() {
		log.Infof("stopping")
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
					if err := t.fsWatcher.Add(filename); err != nil {
						log.Errorf("add dir watch %s: %s", filename, err)
					} else {
						log.Infof("add dir watch %s", filename)
					}
				default:
					if !strings.HasSuffix(filename, ".conf") {
						return nil
					}
					var (
						p   naming.Path
						err error
					)
					if filename == nodeConf {
						// pass
					} else if p, err = cfgFilenameToPath(filename); err != nil {
						log.Warnf("do not watch invalid config file %s: %s", filename, err)
						return nil
					}
					/*
						if err := watcher.Add(filename); err != nil {
							log.Error().Err(err).Msgf("daemon: discover: fs: add file %s: %s", filename, err)
						} else {
							log.Debug().Msgf("daemon: discover: fs: add file %s", filename)
						}
					*/
					log.Debugf("publish msgbus.ConfigFileUpdated config file %s", filename)
					t.PubDebounce(bus, filename, &msgbus.ConfigFileUpdated{Path: p, File: filename}, pubsub.Label{"path", p.String()})
				}
				return nil
			},
		)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cleanup()
		log.Infof("started")
		defer log.Infof("stopped")
		const updateMask = fsnotify.Write | fsnotify.Create | fsnotify.Chmod
		const removeMask = fsnotify.Remove | fsnotify.Rename

		// Add directory watches for:
		//  <etc>/
		//  <var>/node/
		varNodeDir := filepath.Join(rawconfig.Paths.Var, "node")
		nodeFrozenFile := filepath.Join(varNodeDir, "frozen")
		for _, dir := range []string{rawconfig.Paths.Etc, varNodeDir} {
			if err := t.fsWatcher.Add(dir); err != nil {
				log.Errorf("add dir watch %s: %s", dir, err)
			} else {
				log.Infof("add dir watch %s", dir)
			}
		}

		if updated := file.ModTime(nodeFrozenFile); !updated.IsZero() {
			log.Infof("detect %s initially exists", nodeFrozenFile)
			t.PubDebounce(bus, nodeFrozenFile, &msgbus.NodeFrozenFileUpdated{File: nodeFrozenFile, At: updated}, t.labelLocalhost)
		} else {
			log.Infof("detect %s initially absent", nodeFrozenFile)
			t.PubDebounce(bus, nodeFrozenFile, &msgbus.NodeFrozenFileRemoved{File: nodeFrozenFile}, t.labelLocalhost)
		}

		if err := initDirWatches(rawconfig.Paths.Etc); err != nil {
			log.Errorf("init fs watches walking %s: %s", rawconfig.Paths.Etc, err)
		}

		// watcher-events handler loop
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-watcher.Errors:
				log.Errorf("watcher: %s", e)
			case event := <-watcher.Events:
				log.Debugf("event: %s", event)
				filename := event.Name
				switch {
				case strings.HasSuffix(filepath.Dir(filename), "/run"):
					switch {
					case event.Op&fsnotify.Remove != 0:
						log.Debugf("detect removed file %s (%s)", filename, event.Op)
						path, rid, err := runFilenameToPathAndRID(filename)
						if err != nil {
							log.Warnf("failed to parse path and rid from %s: %s", filename, err)
							continue
						}
						t.PubDebounce(bus, filename, &msgbus.RunFileRemoved{File: filename, Path: path, RID: rid, At: time.Now()}, t.labelLocalhost, pubsub.Label{"path", path.String()})
					case event.Op&fsnotify.Create != 0:
						log.Debugf("detect updated file %s (%s)", filename, event.Op)
						path, rid, err := runFilenameToPathAndRID(filename)
						if err != nil {
							log.Warnf("failed to parse path and rid from %s: %s", filename, err)
							continue
						}
						t.PubDebounce(bus, filename, &msgbus.RunFileUpdated{File: filename, Path: path, RID: rid, At: file.ModTime(filename)}, t.labelLocalhost, pubsub.Label{"path", path.String()})
					}
				case strings.HasSuffix(filename, "frozen"):
					if filename != nodeFrozenFile {
						// TODO: track instance frozen flag ? the namespace var is not yet watched
						continue
					}
					switch {
					case event.Op&fsnotify.Remove != 0:
						log.Debugf("detect removed file %s (%s)", filename, event.Op)
						if filename == nodeFrozenFile {
							t.PubDebounce(bus, filename, &msgbus.NodeFrozenFileRemoved{File: filename}, t.labelLocalhost)
						}
					case event.Op&updateMask != 0:
						log.Debugf("detect updated file %s (%s)", filename, event.Op)
						if filename == nodeFrozenFile {
							t.PubDebounce(bus, filename, &msgbus.NodeFrozenFileUpdated{File: filename, At: file.ModTime(filename)}, t.labelLocalhost)
						}
					}
				case strings.HasSuffix(filename, ".conf"):
					var (
						p   naming.Path
						err error
					)
					if filename == nodeConf {
						// pass
					} else if p, err = cfgFilenameToPath(filename); err != nil {
						log.Warnf("can't get associated object path from %s: %s", filename, err)
						continue
					}
					switch {
					case event.Op&removeMask != 0:
						if !file.Exists(filename) {
							log.Debugf("detect removed file %s (%s)", filename, event.Op)
							t.PubDebounce(bus, filename, &msgbus.ConfigFileRemoved{Path: p, File: filename}, pubsub.Label{"path", p.String()})
						}
					case event.Op&updateMask != 0:
						log.Debugf("detect updated file %s (%s)", filename, event.Op)
						t.PubDebounce(bus, filename, &msgbus.ConfigFileUpdated{Path: p, File: filename}, pubsub.Label{"path", p.String()})
					}
				case dirCreated(event):
					if event.Name == "." {
						log.Debugf("skip event %s", event)
						continue
					}
					if err := t.fsWatcher.Add(filename); err != nil {
						log.Errorf("add dir watch %s: %s", filename, err)
					} else {
						log.Infof("add dir watch %s", filename)
					}
					if err := initDirWatches(filename); err != nil {
						log.Errorf("init fs watches walking %s: %s", filename, err)
					}
				case dirRemoved(event):
					if err := t.fsWatcher.Remove(filename); err != nil {
						log.Errorf("remove dir watch %s: %s", filename, err)
					} else {
						log.Infof("remove dir watch %s", filename)
					}
				}
			}
		}
	}()
	return stop, nil
}

func cfgFilenameToPath(filename string) (naming.Path, error) {
	return filenameToPath(filename, rawconfig.Paths.Etc, ".conf")
}

func filenameToPath(filename, prefix, suffix string) (naming.Path, error) {
	svcName := strings.TrimPrefix(filename, prefix+"/")
	svcName = strings.TrimPrefix(svcName, "namespaces/")
	svcName = strings.TrimSuffix(svcName, suffix)
	if len(svcName) == 0 {
		return naming.Path{}, fmt.Errorf("skipped null filename")
	}
	return naming.ParsePath(svcName)
}

func runFilenameToPathAndRID(filename string) (naming.Path, string, error) {
	s := filepath.Dir(filepath.Dir(filename)) // discard the /run suffix
	rid := filepath.Base(s)
	s = filepath.Dir(s) // discard the /<rid> suffix
	s = strings.TrimPrefix(s, rawconfig.Paths.VarNs+"/")
	s = strings.TrimPrefix(s, rawconfig.Paths.Var+"/")
	path, err := naming.ParsePath(s)
	return path, rid, err
}
