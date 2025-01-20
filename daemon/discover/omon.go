package discover

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

func (t *Manager) omon(started chan<- bool) {
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/discover").WithPrefix("daemon: discover: omon: ")
	log.Infof("started")
	omonStarted := make(map[naming.Path]bool)
	defer log.Infof("stopped")
	bus := pubsub.BusFromContext(t.ctx)
	sub := bus.Sub("daemon.discover.omon", t.subQS)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusDone{}, t.labelLocalhost)
	sub.Start()
	started <- true
	defer func() {
		log.Debugf("flushing queue")
		defer log.Debugf("flushed queue")
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
		tC := time.After(t.drainDuration)
		for {
			select {
			case <-tC:
				return
			}
		}
	}()

	startOmon := func(p naming.Path, instanceConfig instance.Config, origin string) {
		log := naming.LogWithPath(log, p)
		log.Infof("%s: start new omon %s", origin, p)
		if err := omon.Start(t.ctx, t.omonSubQS, p, instanceConfig, t.imonStarter); err != nil {
			log.Errorf("%s: start new omon failed for %s: %s", origin, p, err)
			return
		}
		omonStarted[p] = true
	}

	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceConfigUpdated:
				if _, ok := omonStarted[c.Path]; !ok {
					startOmon(c.Path, c.Value, "instance config updated")
				} else {
					// see below ObjectStatusDone on t0 InstanceConfigDeleted,
					// t1 InstanceConfigUpdated
				}
			case *msgbus.ObjectStatusDone:
				delete(omonStarted, c.Path)
				// We must verify if instance config has been recreated, without
				// startOmon on InstanceConfigUpdated event. See following
				// event flow:
				//    t+0 omon: receive InstanceConfigDeleted
				//    t+1 omon: terminating
				//    t+2 discover.omon: receive InstanceConfigUpdated (skipped
				//        because we don't have yet received ObjectStatusDone)
				//    t+3 omon: publish ObjectStatusDone
				//    t+4 discover.omon: receive ObjectStatusDone from t+3
				//                       we have to start new omon
				var lastestConfig *instance.Config
				var lastestConfigFrom string
				for nodename, iConfig := range instance.ConfigData.GetByPath(c.Path) {
					if lastestConfig == nil || iConfig.UpdatedAt.After(lastestConfig.UpdatedAt) {
						lastestConfig = iConfig
						lastestConfigFrom = nodename
					}
				}
				if lastestConfig != nil {
					startOmon(c.Path, *lastestConfig, "object done but instance config exists on "+lastestConfigFrom)
				}
			}
		}
	}
}
