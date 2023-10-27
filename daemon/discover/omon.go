package discover

import (
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

var (
	// SubscriptionQueueSizeOmon is size of "discover.omon" subscription
	SubscriptionQueueSizeOmon = 16000
)

func (d *discover) omon(started chan<- bool) {
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/discover:omon").WithPrefix("daemon: discover: omon: ")
	log.Infof("started")
	defer log.Infof("stopped")
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("discover.omon", pubsub.WithQueueSize(SubscriptionQueueSizeOmon))
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
	sub.Start()
	started <- true
	defer func() {
		log.Debugf("flushing queue")
		defer log.Debugf("flushed queue")
		if err := sub.Stop(); err != nil {
			d.log.Errorf("subscription stop: %s", err)
		}
		tC := time.After(d.drainDuration)
		for {
			select {
			case <-tC:
				return
			case <-d.objectMonitorCmdC:
			}
		}
	}()
	for {
		select {
		case <-d.ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceConfigUpdated:
				s := c.Path.String()
				if _, ok := d.objectMonitor[s]; !ok {
					log.Infof("new object %s", s)
					if err := omon.Start(d.ctx, c.Path, c.Value, d.objectMonitorCmdC, d.imonStarter); err != nil {
						log.Errorf("start %s failed: %s", s, err)
						return
					}
					d.objectMonitor[s] = make(map[string]struct{})
				}
			}
		case i := <-d.objectMonitorCmdC:
			switch c := i.(type) {
			case *msgbus.ObjectStatusDone:
				delete(d.objectMonitor, c.Path.String())
			default:
				log.Errorf("unexpected cmd: %i", i)
			}
		}
	}
}
