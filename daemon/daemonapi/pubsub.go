package daemonapi

import (
	"time"

	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

func AnnounceSub(bus *pubsub.Bus, name string) {
	bus.Pub(msgbus.ClientSub{ApiClient: msgbus.ApiClient{Time: time.Now(), Name: name}})
}

func AnnounceUnSub(bus *pubsub.Bus, name string) {
	bus.Pub(msgbus.ClientUnSub{ApiClient: msgbus.ApiClient{Time: time.Now(), Name: name}})
}
