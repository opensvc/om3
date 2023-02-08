// dns is responsible of the cluster dns zone.
//
// This zone contains the records needed to address a svc with
// cni ip addresses, which are randomly changing on restart.
package dns

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Record struct {
		Name     string `json:"qname"`
		Type     string `json:"qtype"`
		TTL      int    `json:"ttl"`
		Content  string `json:"content"`
		DomainId int    `json:"domain_id"`
	}
	Zone []Record

	stateKey struct {
		path string
		node string
	}

	dns struct {
		// state is a map indexed by object path where the key is a zone fragment regrouping all records created for this object.
		// Using this map layout permits fast records drop on InstanceStatusDeleted.
		// The zone data is obtained by merging all map values.
		state map[stateKey]Zone

		cluster   cluster.Config
		ctx       context.Context
		cancel    context.CancelFunc
		cmdC      chan any
		bus       *pubsub.Bus
		log       zerolog.Logger
		startedAt time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		sub *pubsub.Subscription
	}

	cmdGet struct {
		Name string
		Type string
		resp chan Zone
	}
	cmdGetZone struct {
		resp chan Zone
	}
)

var (
	cmdC chan any
)

func init() {
	cmdC = make(chan any)
}

// Start launches the dns worker goroutine
func Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)

	t := &dns{
		cluster: ccfg.Get(),
		ctx:     ctx,
		cancel:  cancel,
		cmdC:    make(chan any),
		bus:     pubsub.BusFromContext(ctx),
		log:     log.Logger.With().Str("func", "dns").Logger(),
		state:   make(map[stateKey]Zone),
	}

	t.startSubscriptions()

	if err := t.startUDSListener(); err != nil {
		return err
	}

	go func() {
		defer func() {
			msgbus.DropPendingMsg(t.cmdC, time.Second)
			t.sub.Stop()
		}()
		t.worker()
	}()

	// start serving
	cmdC = t.cmdC

	return nil
}

func (t *dns) startSubscriptions() {
	sub := t.bus.Sub("dns")
	for _, last := range sub.AddFilterGetLasts(msgbus.InstanceStatusUpdated{}) {
		t.onInstanceStatusUpdated(last.(msgbus.InstanceStatusUpdated))
	}
	sub.AddFilter(msgbus.InstanceStatusDeleted{})
	sub.AddFilter(msgbus.ClusterConfigUpdated{})
	sub.Start()
	t.sub = sub
}

// worker watch for local dns updates
func (t *dns) worker() {
	defer t.log.Debug().Msg("done")

	t.startedAt = time.Now()

	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch c := i.(type) {
			case msgbus.InstanceStatusUpdated:
				t.onInstanceStatusUpdated(c)
			case msgbus.InstanceStatusDeleted:
				t.onInstanceStatusDeleted(c)
			case msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			}
		case i := <-t.cmdC:
			switch c := i.(type) {
			case cmdGetZone:
				t.onCmdGetZone(c)
			case cmdGet:
				t.onCmdGet(c)
			}
		}
	}
}

func GetZone() Zone {
	c := cmdGetZone{
		resp: make(chan Zone),
	}
	cmdC <- c
	return <-c.resp
}

func (t Zone) Render() string {
	type widthsMap struct {
		Name int
		Type int
		TTL  int
	}
	var (
		widths widthsMap
	)
	for _, record := range t {
		if n := len(record.Name) + 1; n > widths.Name {
			widths.Name = n
		}
		if n := len(record.Type) + 1; n > widths.Type {
			widths.Type = n
		}
		if n := len(fmt.Sprint(record.TTL)) + 1; n > widths.TTL {
			widths.TTL = n
		}
	}
	lines := make([]string, len(t))
	for i, record := range t {
		lineFormat := "%-" + fmt.Sprint(widths.Name) + "s  IN  %-" + fmt.Sprint(widths.Type) + "s %-" + fmt.Sprint(widths.TTL) + "d %s\n"
		lines[i] = fmt.Sprintf(lineFormat, record.Name, record.Type, record.TTL, record.Content)
	}
	sort.Sort(sort.StringSlice(lines))
	return strings.Join(lines, "")
}
