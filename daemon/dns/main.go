// dns is responsible of the cluster dns zone.
//
// This zone contains the records needed to address a svc with
// cni ip addresses, which are randomly changing on restart.
package dns

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/draincommand"
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

		// score stores the node.Stats.Score values, to use as weight in SRV records
		score map[string]uint64

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
		errC
		Name string
		Type string
		resp chan Zone
	}
	cmdGetZone struct {
		errC
		resp chan Zone
	}

	errC draincommand.ErrC
)

var (
	cmdC chan any

	// SubscriptionQueueSize is size of "dns" subscription
	SubscriptionQueueSize = 1000
)

func init() {
	cmdC = make(chan any)
}

// Start launches the dns worker goroutine
func Start(parent context.Context, drainDuration time.Duration) error {
	ctx, cancel := context.WithCancel(parent)

	t := &dns{
		cluster: ccfg.Get(),
		ctx:     ctx,
		cancel:  cancel,
		cmdC:    make(chan any),
		bus:     pubsub.BusFromContext(ctx),
		log:     log.Logger.With().Str("func", "dns").Logger(),
		state:   make(map[stateKey]Zone),
		score:   make(map[string]uint64),
	}

	t.startSubscriptions()

	if err := t.startUDSListener(); err != nil {
		return err
	}

	go func() {
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Error().Err(err).Msg("subscription stop")
			}
			draincommand.Do(t.cmdC, drainDuration)
		}()
		t.worker()
	}()

	// start serving
	cmdC = t.cmdC

	return nil
}

func (t *dns) startSubscriptions() {
	sub := t.bus.Sub("dns", pubsub.WithQueueSize(SubscriptionQueueSize))
	sub.AddFilter(&msgbus.InstanceStatusUpdated{})
	sub.AddFilter(&msgbus.InstanceStatusDeleted{})
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.NodeStatsUpdated{})
	sub.Start()
	t.sub = sub
}

// worker watch for local dns updates
func (t *dns) worker() {
	defer t.log.Debug().Msg("done")

	for _, v := range instance.StatusData.GetAll() {
		t.onInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{Node: v.Node, Path: v.Path, Value: *v.Value})
	}

	t.startedAt = time.Now()

	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceStatusUpdated:
				t.onInstanceStatusUpdated(c)
			case *msgbus.InstanceStatusDeleted:
				t.onInstanceStatusDeleted(c)
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			case *msgbus.NodeStatsUpdated:
				t.onNodeStatsUpdated(c)
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
	err := make(chan error, 1)
	c := cmdGetZone{
		errC: err,
		resp: make(chan Zone),
	}
	cmdC <- c
	if <-err != nil {
		return Zone{}
	}
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
