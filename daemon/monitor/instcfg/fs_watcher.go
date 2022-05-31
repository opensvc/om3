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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		o.log.Error().Err(err).Msgf("NewWatcher")
		return err
	}
	if err = watcher.Add(o.filename); err != nil {
		o.log.Error().Err(err).Msgf("watcher add")
		if err := watcher.Close(); err != nil {
			o.log.Error().Err(err).Msgf("watcher Close")
		}
		return err
	}
	go func() {
		defer func() {
			if err := watcher.Close(); err != nil {
				o.log.Error().Err(err).Msgf("watcher Close")
			}
		}()
		o.log.Info().Msgf("watching file %s", o.filename)
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
							o.log.Info().Msgf("file %s removed", o.filename)
							o.cmdC <- moncmd.New(moncmd.CfgFileRemoved{})
							return
						} else {
							o.log.Info().Msgf("re-add watch on %s", o.filename)
							if err := watcher.Add(o.filename); err != nil {
								o.log.Error().Err(err).Msgf("re-add watch on %s", o.filename)
							}
						}
					}
					o.log.Info().Msgf("file %s updated", o.filename)
					o.cmdC <- moncmd.New(moncmd.CfgFileUpdated{})
				}
			}
		}
	}()
	return nil
}
