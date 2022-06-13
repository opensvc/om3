package instcfg

import (
	"time"

	"github.com/fsnotify/fsnotify"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/file"
)

var (
	delayExistAfterRemove = 100 * time.Millisecond
)

func (o *instCfg) watchFile() error {
	log := o.log.With().Str("func", "instcfg.watchFile").Str("cfgfile", o.filename).Logger()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("NewWatcher")
		return err
	}
	if err = watcher.Add(o.filename); err != nil {
		log.Error().Err(err).Msg("watcher.Add")
		if err := watcher.Close(); err != nil {
			log.Error().Err(err).Msg("Close")
		}
		return err
	}
	go func() {
		defer func() {
			if err := watcher.Close(); err != nil {
				log.Error().Err(err).Msg("defer Close")
			}
		}()
		log.Debug().Msg("watching events")
		const updateMask = fsnotify.Write | fsnotify.Create | fsnotify.Remove | fsnotify.Rename | fsnotify.Chmod
		const needReAddMask = fsnotify.Remove | fsnotify.Rename
		for {
			select {
			case <-o.ctx.Done():
				return
			case event := <-watcher.Events:
				if event.Op&updateMask != 0 {
					if event.Op&needReAddMask != 0 {
						time.Sleep(delayExistAfterRemove)
						if !file.Exists(o.filename) {
							log.Info().Msg("file removed")
							o.cmdC <- moncmd.New(moncmd.CfgFileRemoved{})
							return
						} else {
							log.Debug().Msg("re-add watch")
							if err := watcher.Add(o.filename); err != nil {
								log.Error().Err(err).Msg("watcher.Add")
							}
						}
					}
					log.Debug().Msg("file updated")
					o.cmdC <- moncmd.New(moncmd.CfgFileUpdated{})
				}
			}
		}
	}()
	return nil
}
