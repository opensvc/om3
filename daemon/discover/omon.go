package discover

import (
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

func (t *Manager) omon(started chan<- bool) {
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/discover").WithPrefix("daemon: discover: omon: ")
	log.Infof("started")
	defer log.Infof("stopped")
	bus := pubsub.BusFromContext(t.ctx)
	sub := bus.Sub("discover.omon", t.subQS)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
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
			case <-t.objectMonitorCmdC:
			}
		}
	}()
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceConfigUpdated:
				s := c.Path.String()
				if _, ok := t.objectMonitor[s]; !ok {
					log.Infof("new object %s", s)
					if err := omon.Start(t.ctx, t.omonSubQS, c.Path, c.Value, t.objectMonitorCmdC, t.imonStarter); err != nil {
						log.Errorf("start %s failed: %s", s, err)
						return
					}
					t.objectMonitor[s] = make(map[string]struct{})
				}
			}
		case i := <-t.objectMonitorCmdC:
			switch c := i.(type) {
			case *msgbus.ObjectStatusDone:
				delete(t.objectMonitor, c.Path.String())
			default:
				log.Errorf("unexpected cmd: %i", i)
			}
		}
	}
}
