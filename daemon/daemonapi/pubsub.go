package daemonapi

import (
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func announceSub(bus *pubsub.Bus, name string) {
	bus.Pub(&msgbus.ClientSub{Time: time.Now(), Name: name})
}

func announceUnSub(bus *pubsub.Bus, name string) {
	bus.Pub(&msgbus.ClientUnSub{Time: time.Now(), Name: name})
}
