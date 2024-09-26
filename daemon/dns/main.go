// Package dns is responsible for the cluster dns zone management.
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
	"sync"
	"time"

	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/draincommand"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Record struct {
		Name     string `json:"qname"`
		Type     string `json:"qtype"`
		TTL      int    `json:"ttl"`
		Content  string `json:"content"`
		DomainID int    `json:"domain_id"`
	}
	Zone []Record

	stateKey struct {
		path string
		node string
	}

	Manager struct {
		drainDuration time.Duration

		// state is a map indexed by object path where the key is a zone fragment regrouping all records created for this object.
		// Using this map layout permits fast records drop on InstanceStatusDeleted.
		// The zone data is obtained by merging all map values.
		state map[stateKey]Zone

		// score stores the node.Stats.Score values, to use as weight in SRV records
		score map[string]uint64

		cluster   clusterdump.Config
		ctx       context.Context
		cancel    context.CancelFunc
		cmdC      chan any
		bus       *pubsub.Bus
		log       *plog.Logger
		startedAt time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		wg sync.WaitGroup

		// status is the daemon dns subsystem status
		//    - state: "running" when started, or ""
		//    - configured_at: is the time of last chown on dns socket
		//    - updated_at: is the time of last update (chown on dns socket or NameServers)
		//    - NameServers: is the list of name servers from the cluster dns configuration
		status daemonsubsystem.Dns

		localhost string
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
)

func init() {
	cmdC = make(chan any)
}

func NewManager(d time.Duration, subQS pubsub.QueueSizer) *Manager {
	return &Manager{
		cmdC:          make(chan any),
		drainDuration: d,
		state:         make(map[stateKey]Zone),
		score:         make(map[string]uint64),
		subQS:         subQS,

		status: daemonsubsystem.Dns{
			Status:      daemonsubsystem.Status{ID: "dns", CreatedAt: time.Now()},
			Nameservers: make([]string, 0),
		},
		localhost: hostname.Hostname(),
	}
}

// Start launches the dns worker goroutine
func (t *Manager) Start(parent context.Context) error {
	t.log = plog.NewDefaultLogger().WithPrefix("daemon: dns: ").Attr("pkg", "daemon/dns")
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.bus = pubsub.BusFromContext(t.ctx)

	t.status.State = "running"

	t.startSubscriptions()
	t.cluster = *clusterdump.ConfigData.Get()

	if err := t.startUDSListener(); err != nil {
		return err
	}

	t.wg.Add(1)
	go func() {
		t.wg.Done()
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Errorf("subscription stop: %s", err)
			}
			t.status.State = ""
			t.publishSubsystemDnsUpdated()
			draincommand.Do(t.cmdC, t.drainDuration)
		}()
		t.worker()
	}()

	// start serving
	cmdC = t.cmdC

	t.log.Infof("started")
	return nil
}

func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) startSubscriptions() {
	sub := t.bus.Sub("daemon.dns", t.subQS)
	sub.AddFilter(&msgbus.InstanceStatusUpdated{})
	sub.AddFilter(&msgbus.InstanceStatusDeleted{})
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.NodeStatsUpdated{})
	sub.Start()
	t.sub = sub
}

// worker watch for local dns updates
func (t *Manager) worker() {
	defer t.log.Debugf("done")

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

func (t *Manager) publishSubsystemDnsUpdated() {
	t.status.UpdatedAt = time.Now()
	t.status.Nameservers = append([]string{}, t.cluster.DNS...)
	daemonsubsystem.DataDns.Set(t.localhost, t.status.DeepCopy())
	t.bus.Pub(&msgbus.DaemonDnsUpdated{Node: t.localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", t.localhost})
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
