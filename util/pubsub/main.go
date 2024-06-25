// Package pubSub implements simple pub-sub bus with filtering by labels
//
// Example:
//    import (
//    	"context"
//    	"fmt"
//
//    	"opensvc.com/opensvc/util/pubsub"
//    )
//
//    func main() {
//      ctx, cancel := context.WithCancel(context.Background())
//      defer cancel()
//
//  	// Start the pub-sub
//      c := pubSub.Start(ctx, "pub-sub example")
//
//    	// register a subscription that watch all string-typed data
//    	sub := pubSub.Sub(c, pubSub.Subscription{Name: "watch all", "template string"})
//	defer sub.Stop()
//
//    	go func() {
//		select {
//		case i := <-sub.C:
//			fmt.Printf("detected from subscription 2: value '%s' have been published\n", i)
//		}
//	}()
//
//    	// publish a string message with some labels
//    	pubSub.Pub(c, "a dataset", Label{"ns": "ns1"}, Label{"op": "create"})
//
//    	// publish a string message with different labels
//    	pubSub.Pub(c, "another dataset", Label{"ns", "ns2"}, Label{"op", "update"})
//    }
//

package pubsub

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/opensvc/om3/util/durationlog"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/opensvc/om3/util/xmap"
)

type (
	contextKey int
)

const (
	busContextKey contextKey = 0
)

type (
	// Labels allow message routing filtering based on key/value matching
	Labels map[string]string

	// Label is a {key, val} array
	Label [2]string

	// subscriptions is a hash of subscription indexed by multiple lookup criteria
	subscriptionMap map[string]map[uuid.UUID]any

	filter struct {
		labels   Labels
		dataType string
	}

	filters []filter

	Subscription struct {
		filters filters
		name    string
		id      uuid.UUID
		bus     *Bus

		// q is a private channel pushing to C with timeout
		q chan any

		// C is the channel exposed to the subscriber for polling
		C chan any

		// when non 0, the subscription is stopped if the push timeout exceeds timeout
		timeout time.Duration

		// cancel defines the subscription canceler
		cancel context.CancelFunc

		// drainChanDuration is the max duration during draining channels
		drainChanDuration time.Duration

		queuedMin  uint64
		queuedMax  uint64
		queuedSize uint64
		queued     atomic.Uint64
	}

	cmdPub struct {
		labels   Labels
		dataType string
		data     any
		resp     chan<- bool
	}

	cmdSubAddFilter struct {
		id       uuid.UUID
		labels   Labels
		dataType string
		resp     chan<- error
	}

	cmdSubDelFilter struct {
		id       uuid.UUID
		labels   Labels
		dataType string
		resp     chan<- error
	}

	cmdSub struct {
		name      string
		resp      chan<- *Subscription
		timeout   time.Duration
		queueSize uint64
	}

	cmdUnsub struct {
		id  uuid.UUID
		err chan<- error
	}

	Bus struct {
		sync.WaitGroup
		name        string
		cmdC        chan any
		cancel      func()
		log         *plog.Logger
		ctx         context.Context
		subs        map[uuid.UUID]*Subscription
		subMap      subscriptionMap
		beginNotify chan uuid.UUID
		endNotify   chan uuid.UUID
		started     bool

		// drainChanDuration is the max duration during draining private and exposed
		// channel
		drainChanDuration time.Duration

		// default queue size for subscriptions
		subQueueSize uint64

		// panicOnFullQueueGraceTime is the grace time duration we have to wait
		// before panic when a subscription with no timeout has reached its
		// maximum queue size.
		// Default value (zero) disable panic on full queue feature.
		panicOnFullQueueGraceTime time.Duration
	}

	stringer interface {
		String() string
	}

	Msg struct {
		Labels Labels `json:"labels"`
	}

	Messager interface {
		AddLabels(...Label)
		GetLabels() Labels
	}
)

func NewLabels(l ...string) Labels {
	var k string
	m := make(Labels)
	for i, s := range l {
		switch i % 2 {
		case 0:
			k = s
		case 1:
			m[k] = s
		}
	}
	return m
}

func (p *Msg) GetLabels() Labels {
	m := make(Labels)
	if p.Labels == nil {
		return m
	}
	for k, v := range p.Labels {
		m[k] = v
	}
	return m
}

func (p *Msg) AddLabels(l ...Label) {
	if len(l) == 0 {
		return
	}
	if p.Labels == nil {
		p.Labels = make(Labels)
	}
	for _, e := range l {
		p.Labels[e[0]] = e[1]
	}
}

var (
	// defaultSubscriptionQueueSize is default size of internal subscription queue
	defaultSubscriptionQueueSize uint64 = 4000

	cmdDurationWarn    = time.Second
	notifyDurationWarn = 5 * time.Second

	// defaultDrainChanDuration is the default duration to wait while draining channel
	defaultDrainChanDuration = 10 * time.Millisecond

	uint64Incr = uint64(1)

	publicationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_pubsub_publication_total",
			Help: "The total number of pubsub publications",
		},
		[]string{"kind"})

	publicationPushedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_pubsub_publication_pushed_total",
			Help: "The total number of pubsub publications pushed",
		},
		[]string{"filterkey"})

	subscriptionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_pubsub_subscription_total",
			Help: "The total number of pubsub subscriptions",
		},
		[]string{"operation"})

	subscriptionFilterTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "opensvc_pubsub_subscription_filter_total",
			Help: "The total number of pubsub subscription filter operations",
		},
		[]string{"kind"})
)

// Key returns labelMap key as a string
// with ordered label names
func (t Labels) Key() string {
	s := ""
	var sortKeys []string
	for key := range t {
		sortKeys = append(sortKeys, key)
	}
	sort.Strings(sortKeys)
	for _, key := range sortKeys {
		s += "{" + key + "=" + t[key] + "}"
	}
	return s
}

// Keys returns all the permutations of all lengths of the labels
// ex:
//
//	keys of l1=foo l2=foo l3=foo:
//	 {l1=foo}
//	 {l2=foo}
//	 {l3=foo}
//	 {l1=foo}{l2=foo}
//	 {l1=foo}{l3=foo}
//	 {l2=foo}{l3=foo}
//	 {l2=foo}{l1=foo}
//	 {l3=foo}{l1=foo}
//	 {l3=foo}{l2=foo}
//	 {l1=foo}{l2=foo}{l3=foo}
//	 {l1=foo}{l3=foo}{l2=foo}
//	 {l2=foo}{l1=foo}{l3=foo}
//	 {l2=foo}{l3=foo}{l1=foo}
//	 {l3=foo}{l1=foo}{l2=foo}
//	 {l3=foo}{l2=foo}{l1=foo}
func (t Labels) Keys() []string {
	m := map[string]any{"": nil}
	keys := xmap.Keys(t)
	total := len(keys)
	for _, keys := range stringslice.Permute(keys) {
		for i := 0; i < total; i++ {
			for _, perm := range stringslice.Permute(keys[:i+1]) {
				s := ""
				for _, key := range perm {
					s += "{" + key + "=" + t[key] + "}"
				}
				m[s] = nil
			}
		}
	}
	return xmap.Keys(m)
}

// FilterFmt returns a string that identify a filter
func FilterFmt(kind string, labels ...Label) string {
	return fmtKey(kind, newLabels(labels...))
}

func (t Labels) Is(labels Labels) bool {
	m1 := make(map[string]any)
	m2 := make(map[string]any)
	for k, v := range t {
		m1[fmt.Sprintf("%#v", []string{k, v})] = nil
	}
	for k, v := range labels {
		m2[fmt.Sprintf("%#v", []string{k, v})] = nil
	}
	for l1 := range m1 {
		if _, ok := m2[l1]; !ok {
			return false
		} else {
			delete(m2, l1)
		}
	}
	for l2 := range m2 {
		if _, ok := m1[l2]; !ok {
			return false
		}
	}
	return true
}

func newLabels(labels ...Label) Labels {
	m := make(Labels)
	for _, label := range labels {
		m[label[0]] = label[1]
	}
	return m
}

// NewBus allocate and runs a new Bus and return a pointer
func NewBus(name string) *Bus {
	b := &Bus{}
	b.name = name
	b.cmdC = make(chan any)
	b.beginNotify = make(chan uuid.UUID)
	b.endNotify = make(chan uuid.UUID)
	b.log = plog.NewDefaultLogger().WithPrefix(fmt.Sprintf("%s: pubsub: ", name)).Attr("pkg", "util/pubsub").Attr("bus_name", name)
	b.drainChanDuration = defaultDrainChanDuration
	b.subQueueSize = defaultSubscriptionQueueSize
	return b
}

func (b *Bus) Name() string {
	return b.name
}

func (b *Bus) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	started := make(chan bool)
	b.subs = make(map[uuid.UUID]*Subscription)
	b.subMap = make(subscriptionMap)

	b.Add(1)
	go func() {
		defer b.Done()

		watchDuration := &durationlog.T{Log: *b.log}
		watchDurationCtx, watchDurationCancel := context.WithCancel(context.Background())
		defer watchDurationCancel()
		var beginCmd = make(chan any)
		var endCmd = make(chan bool)
		b.Add(1)
		go func() {
			defer b.Done()
			watchDuration.WarnExceeded(watchDurationCtx, beginCmd, endCmd, cmdDurationWarn, "msg")
		}()

		b.Add(1)
		go func() {
			defer b.Done()
			b.warnExceededNotification(watchDurationCtx, notifyDurationWarn)
		}()

		started <- true
		for {
			select {
			case <-b.ctx.Done():
				return
			case cmd := <-b.cmdC:
				beginCmd <- cmd
				switch c := cmd.(type) {
				case cmdPub:
					b.onPubCmd(c)
				case cmdSubAddFilter:
					b.onSubAddFilter(c)
				case cmdSubDelFilter:
					b.onSubDelFilter(c)
				case cmdSub:
					b.onSubCmd(c)
				case cmdUnsub:
					b.onUnsubCmd(c)
				}
				endCmd <- true
			}
		}
	}()
	b.started = <-started
	b.log.Infof("bus started")
}

// SetDrainChanDuration overrides defaultDrainChanDuration for not yet started bus.
//
// It panics if called on started bus.
func (b *Bus) SetDrainChanDuration(duration time.Duration) {
	if b.started {
		panic("can't set drain channel duration on started bus")
	}
	b.drainChanDuration = duration
}

// SetDefaultSubscriptionQueueSize overrides the default queue size of subscribers for not yet started bus.
//
// It panics if called on started bus.
func (b *Bus) SetDefaultSubscriptionQueueSize(i uint64) {
	if b.started {
		panic("can't set default subscription queue size on started bus")
	}
	b.subQueueSize = i
}

// SetPanicOnFullQueue enable panic after grace time on subscriptions with
// no timeout has reached subscription maximum queue size.
// Zero graceTime disable panic on full queue feature.
//
// It panics if called on started bus.
func (b *Bus) SetPanicOnFullQueue(graceTime time.Duration) {
	if b.started {
		panic("can't set panic on full queue on started bus")
	}
	b.panicOnFullQueueGraceTime = graceTime
}

func (b *Bus) onSubCmd(c cmdSub) {
	id := uuid.New()
	sub := &Subscription{
		name:    c.name,
		C:       make(chan any, c.queueSize),
		q:       make(chan any, c.queueSize),
		id:      id,
		timeout: c.timeout,
		bus:     b,

		drainChanDuration: b.drainChanDuration,
		queuedMax:         c.queueSize / 32,
		queuedMin:         c.queueSize / 32,
		queuedSize:        c.queueSize,
	}
	b.subs[id] = sub
	c.resp <- sub
	b.log.Debugf("subscribe %s timeout %s queueSize %d", sub.name, c.timeout, c.queueSize)
	subscriptionTotal.With(prometheus.Labels{"operation": "create"}).Inc()
}

func (b *Bus) onUnsubCmd(c cmdUnsub) {
	sub, ok := b.subs[c.id]
	if !ok {
		c.err <- ErrSubscriptionIDNotFound{id: c.id}
		return
	}
	sub.cancel()
	delete(b.subs, c.id)
	b.subMap.Del(c.id, sub.keys()...)
	select {
	case <-b.ctx.Done():
		c.err <- b.ctx.Err()
	case c.err <- nil:
	}
	b.log.Debugf("unsubscribe %s", sub.name)
	subscriptionTotal.With(prometheus.Labels{"operation": "delete"}).Inc()
}

func (b *Bus) onPubCmd(c cmdPub) {
	for _, toFilterKey := range c.keys() {
		// search publication that listen on one of cmdPub.keys
		if subIDMap, ok := b.subMap[toFilterKey]; ok {
			for subID := range subIDMap {
				sub, ok := b.subs[subID]
				if !ok {
					// This should not happen
					b.log.Warnf("filter key %s has a dead subscription %s", toFilterKey, subID)
					continue
				}
				b.log.Debugf("route %s to %s", c, sub)
				queueLen := sub.queued.Add(1)
				sub.q <- c.data
				publicationPushedTotal.With(prometheus.Labels{"filterkey": toFilterKey}).Inc()
				if queueLen >= sub.queuedSize && sub.timeout == 0 && b.panicOnFullQueueGraceTime > 0 {
					// TODO: increase queue size instead of panic ?
					err := fmt.Errorf("subscription %s has reached maximum %d of %d queued pending message, "+
						"allow %s for decrease before panic", sub.name, queueLen, sub.queuedSize, b.panicOnFullQueueGraceTime)
					b.log.Warnf("%s", err)
					go func() {
						<-time.After(b.panicOnFullQueueGraceTime)
						if sub.queued.Load() >= sub.queuedSize {
							err := fmt.Errorf("maximum queued pending message for subscription %s %d of %d", sub.name, queueLen, sub.queuedSize)
							b.log.Errorf("panic: %s", err)
							panic(err)
						} else {
							b.log.Infof("abort panic: subscription %s has leave maximum %d of %d queued pending message", sub.name, sub.queued.Load(), sub.queuedSize)
						}
					}()
				}
				if queueLen > sub.queuedMax {
					inc := sub.queuedSize / 4
					previous := sub.queuedMax
					sub.queuedMax += inc
					left := sub.queuedSize - sub.queuedMax
					level := "debug"
					if left < inc {
						// 3/4 full
						level = "warn"
						b.log.Errorf("subscription %s has reached high %d queued pending message, increase threshold %d -> %d of limit %d", sub.name, queueLen, previous, sub.queuedMax, sub.queuedSize)
					} else if left < sub.queuedSize/2 {
						// 1/2 full
						level = "info"
						b.log.Warnf("subscription %s has reached high %d queued pending message, increase threshold %d -> %d of limit %d", sub.name, queueLen, previous, sub.queuedMax, sub.queuedSize)
					} else {
						b.log.Debugf("subscription %s has reached high %d queued pending message, increase threshold %d -> %d of limit %d", sub.name, queueLen, previous, sub.queuedMax, sub.queuedSize)
					}
					go sub.bus.Pub(&SubscriptionQueueThreshold{Name: sub.name, ID: sub.id, Count: queueLen, From: previous, To: sub.queuedMax, Limit: sub.queuedSize}, Label{"counter", ""}, Label{"level", level})
				} else if queueLen > sub.queuedMin && queueLen < sub.queuedMax/4 {
					previous := sub.queuedMax
					sub.queuedMax /= 8
					left := sub.queuedSize - sub.queuedMax
					level := "debug"
					if left < sub.queuedSize/2 {
						// 1/2 full
						level = "info"
						b.log.Infof("subscription %s has reached low %d queued pending message, decrease threshold %d -> %d of limit %d", sub.name, queueLen, previous, sub.queuedMax, sub.queuedSize)
					} else {
						b.log.Debugf("subscription %s has reached low %d queued pending message, decrease threshold %d -> %d of limit %d", sub.name, queueLen, previous, sub.queuedMax, sub.queuedSize)
					}
					go sub.bus.Pub(&SubscriptionQueueThreshold{Name: sub.name, ID: sub.id, Count: queueLen, From: previous, To: sub.queuedMax, Limit: sub.queuedSize}, Label{"counter", ""}, Label{"level", level})
				}
			}
		}
	}
	c.resp <- true
	publicationTotal.With(prometheus.Labels{"kind": c.dataType}).Inc()
}

func (b *Bus) onSubAddFilter(c cmdSubAddFilter) {
	sub, ok := b.subs[c.id]
	if !ok {
		// TODO c.resp should be error here
		c.resp <- nil
		return
	}
	sub.filters = append(sub.filters, filter{
		dataType: c.dataType,
		labels:   c.labels,
	})
	b.subs[c.id] = sub
	b.subMap.Del(c.id, ":")
	b.subMap.Add(c.id, sub.keys()...)
	c.resp <- nil
}

func (b *Bus) onSubDelFilter(c cmdSubDelFilter) {
	sub, ok := b.subs[c.id]
	if !ok {
		c.resp <- nil
		return
	}
	filters := make(filters, 0)
	for _, f := range sub.filters {
		if f.dataType == c.dataType && f.labels.Is(c.labels) {
			continue
		} else {
			filters = append(filters, f)
		}
	}
	sub.filters = filters
	b.subs[c.id] = sub
	b.subMap.Del(c.id, ":")
	b.subMap.Add(c.id, sub.keys()...)
	c.resp <- nil
}

func (b *Bus) drain() {
	b.log.Infof("draining the message bus")
	defer b.log.Infof("drained")
	i := 0
	tC := time.After(b.drainChanDuration)
	for {
		select {
		case <-b.cmdC:
			i++
		case <-tC:
			b.log.Infof("drained dropped %d pending messages from the bus on stop", i)
			return
		}
	}
}

func (b *Bus) Stop() {
	if b == nil {
		return
	}
	if b.cancel != nil {
		f := b.cancel
		b.cancel = nil
		f()
		b.Wait()
		go b.drain()
		b.log.Infof("stopped")
	}
}

// Pub posts a new Publication on the bus.
// The labels are added to existing v labels, so a subscriber can retrieve message
// publication labels from the received message.
func (b *Bus) Pub(v Messager, labels ...Label) {
	done := make(chan bool)
	v.AddLabels(labels...)
	op := cmdPub{
		labels: v.GetLabels(),
		data:   v,
		resp:   done,
	}
	dataType := reflect.TypeOf(v)
	if dataType != nil {
		op.dataType = dataType.String()
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return
	}
	<-done
}

type (
	Timeouter interface {
		timout() time.Duration
	}

	QueueSizer interface {
		queueSize() uint64
	}
)

type (
	WithQueueSize uint64
	Timeout       time.Duration
)

// queueSize implements QueueSizer for WithQueueSize
func (t WithQueueSize) queueSize() uint64 {
	return uint64(t)
}

// timout implements Timeouter for Timeout
func (t Timeout) timout() time.Duration {
	return time.Duration(t)
}

// Sub function requires a new Subscription on the bus.
//
// Used options: Timeouter, QueueSizer
//
// when Timeouter, it sets the subscriber timeout to pull each message,
// subscriber with exceeded timeout notification are automatically dropped, and SubscriptionError
// message is sent on bus.
// defaults is no timeout
//
// when QueueSizer, it sets the subscriber queue size.
// default value is bus dependent (see SetDefaultSubscriptionQueueSize)
func (b *Bus) Sub(name string, options ...interface{}) *Subscription {
	respC := make(chan *Subscription)
	op := cmdSub{
		name:      name,
		resp:      respC,
		queueSize: b.subQueueSize,
	}

	for _, opt := range options {
		switch v := opt.(type) {
		case Timeouter:
			op.timeout = v.timout()
		case QueueSizer:
			op.queueSize = v.queueSize()
		default:
			panic("invalid option type: " + reflect.TypeOf(opt).String())
		}
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return nil
	}
	return <-respC
}

// Unsub function remove a subscription
func (b *Bus) unsub(sub *Subscription) error {
	errC := make(chan error)
	op := cmdUnsub{
		id:  sub.id,
		err: errC,
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return b.ctx.Err()
	}
	defer subscriptionTotal.With(prometheus.Labels{"operation": "stop"}).Inc()
	return <-errC
}

// warnExceededNotification log when notify duration between <-begin and <-end exceeds maxDuration.
func (b *Bus) warnExceededNotification(ctx context.Context, maxDuration time.Duration) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	pending := make(map[uuid.UUID]time.Time)
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-b.beginNotify:
			pending[id] = time.Now()
		case id := <-b.endNotify:
			delete(pending, id)
		case <-ticker.C:
			now := time.Now()
			for id, begin := range pending {
				if now.Sub(begin) > maxDuration {
					duration := time.Now().Sub(begin).Seconds()
					sub := b.subs[id]
					b.log.Warnf("waited %.02fs over %s for %s", duration, maxDuration, sub.name)
				}
			}
		}
	}
}

// ContextWithBus stores the bus in the context and returns the new context.
func ContextWithBus(ctx context.Context, bus *Bus) context.Context {
	return context.WithValue(ctx, busContextKey, bus)
}

func BusFromContext(ctx context.Context) *Bus {
	if bus, ok := ctx.Value(busContextKey).(*Bus); ok {
		return bus
	}
	panic("unable to retrieve pubsub bus from context")
}

func (cmd cmdSubAddFilter) String() string {
	s := fmt.Sprintf("add subscription %s filter type %s", cmd.id, cmd.dataType)
	if len(cmd.labels) > 0 {
		s += " with " + cmd.labels.String()
	}
	return s
}

func (cmd cmdSub) String() string {
	s := fmt.Sprintf("subscribe '%s'", cmd.name)
	return s
}

func (cmd cmdUnsub) String() string {
	return fmt.Sprintf("unsubscribe key %s", cmd.id)
}

func (t Labels) String() string {
	if len(t) == 0 {
		return ""
	}
	s := "labels"
	for k, v := range t {
		s += fmt.Sprintf(" %s=%s", k, v)
	}
	return s
}

// Drain dequeues exposed channel.
//
// Drain is automatically called during sub.Stop()
func (sub *Subscription) Drain() {
	tC := time.NewTicker(sub.drainChanDuration)
	defer tC.Stop()
	for {
		select {
		case <-sub.C:
		case <-tC.C:
			return
		}
	}
}

// drain dequeues any pending message from private channel
func (sub *Subscription) drain() {
	ticker := time.NewTicker(sub.drainChanDuration)
	defer ticker.Stop()
	for {
		select {
		case <-sub.q:
			sub.queued.Add(-uint64Incr)
		case <-ticker.C:
			return
		}
	}
}

// keys return [] of sub filterkeys
//
//	[]string{
//	        "<Type>:",  // a filter of <Type> without labels
//	        "<Type>:{<name>:<value>}{<name>:<value>}....
//	}
func (sub *Subscription) keys() []string {
	if len(sub.filters) == 0 {
		return []string{":"}
	}
	l := make([]string, len(sub.filters))
	for i, f := range sub.filters {
		l[i] = f.key()
	}
	return l
}

func (pub cmdPub) String() string {
	var dataS string
	switch data := pub.data.(type) {
	case stringer:
		dataS = data.String()
	default:
		dataS = "type " + pub.dataType
	}
	s := fmt.Sprintf("publication %s", dataS)
	if len(pub.labels) > 0 {
		s += " with " + pub.labels.String()
	}
	return s
}

func (pub cmdPub) key() string {
	return fmtKey(pub.dataType, pub.labels)
}

func (pub cmdPub) keys() []string {
	return pubKeys(pub.dataType, pub.labels)
}

func (t filter) key() string {
	return fmtKey(t.dataType, t.labels)
}

func fmtKey(dataType string, labels Labels) string {
	return dataType + ":" + labels.Key()
}

func pubKeys(dataType string, labels Labels) []string {
	return append(
		keys(dataType, labels),
		keys("", labels)...,
	)
}

func keys(dataType string, labels Labels) []string {
	var l []string
	if len(labels) == 0 {
		return []string{dataType + ":"}
	}
	for _, key := range labels.Keys() {
		l = append(l, dataType+":"+key)
	}
	return l
}

func (sub *Subscription) String() string {
	s := fmt.Sprintf("subscription '%s'", sub.name)
	for _, f := range sub.filters {
		if f.dataType != "" {
			s += " on msg type " + f.dataType
		} else {
			s += " on msg type *"
		}
		if len(f.labels) > 0 {
			s += " with " + f.labels.String()
		}
	}
	return s
}

func (sub *Subscription) DelFilter(v any, labels ...Label) {
	respC := make(chan error)
	op := cmdSubDelFilter{
		id:     sub.id,
		labels: newLabels(labels...),
		resp:   respC,
	}
	dataType := reflect.TypeOf(v)
	if dataType != nil {
		op.dataType = dataType.String()
	}
	select {
	case sub.bus.cmdC <- op:
	case <-sub.bus.ctx.Done():
		return
	}
	<-respC
	subscriptionFilterTotal.With(prometheus.Labels{"kind": op.dataType}).Inc()
}

func (sub *Subscription) AddFilter(v any, labels ...Label) {
	respC := make(chan error)
	op := cmdSubAddFilter{
		id:     sub.id,
		labels: newLabels(labels...),
		resp:   respC,
	}
	dataType := reflect.TypeOf(v)
	if dataType != nil {
		op.dataType = dataType.String()
	}
	select {
	case sub.bus.cmdC <- op:
	case <-sub.bus.ctx.Done():
		return
	}
	<-respC
	subscriptionFilterTotal.With(prometheus.Labels{"kind": op.dataType}).Inc()
}

func (sub *Subscription) Start() {
	if len(sub.filters) == 0 {
		// listen all until AddFilter is called
		sub.AddFilter(nil)
	}
	ctx, cancel := context.WithCancel(sub.bus.ctx)
	sub.cancel = cancel
	started := make(chan bool)
	sub.bus.Add(1)
	go func() {
		sub.bus.Done()
		defer sub.cancel()
		defer sub.drain()
		started <- true
		for {
			select {
			case <-ctx.Done():
				return
			case i := <-sub.q:
				sub.queued.Add(-uint64Incr)
				select {
				case <-ctx.Done():
					// sub, or bus is done
					return
				case sub.bus.beginNotify <- sub.id:
				}
				if err := sub.push(i); err != nil {
					// the subscription got push error, cancel it and ask for unsubscribe
					sub.bus.log.Warnf("%s error: %s. stop subscription", sub, err)
					go sub.bus.Pub(&SubscriptionError{Name: sub.name, ID: sub.id, ErrS: err.Error()})
					sub.cancel()
					go func() {
						if err := sub.Stop(); err != nil {
							sub.bus.log.Warnf("stop %s: %s", sub, err)
						}
					}()
					select {
					case <-sub.bus.ctx.Done():
						return
					case sub.bus.endNotify <- sub.id:
					}
				}
				select {
				case <-sub.bus.ctx.Done():
					return
				case sub.bus.endNotify <- sub.id:
				}
			}
		}
	}()
	<-started
	subscriptionTotal.With(prometheus.Labels{"operation": "start"}).Inc()
}

// Stop closes the subscription and deueues private and exposed subscription channels
func (sub *Subscription) Stop() error {
	go sub.Drain()
	return sub.bus.unsub(sub)
}

func (sub *Subscription) push(i any) error {
	if sub.timeout == 0 {
		sub.C <- i
	} else {
		timer := time.NewTimer(sub.timeout)
		select {
		case sub.C <- i:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			return fmt.Errorf("push exceed timeout %s", sub.timeout)
		}
	}
	return nil
}

func (subM subscriptionMap) Del(id uuid.UUID, keys ...string) {
	for _, key := range keys {
		if m, ok := subM[key]; ok {
			delete(m, id)
			subM[key] = m
		}
	}
}

func (subM subscriptionMap) Add(id uuid.UUID, keys ...string) {
	for _, key := range keys {
		if m, ok := subM[key]; ok {
			m[id] = nil
			subM[key] = m
		} else {
			m = map[uuid.UUID]any{id: nil}
			subM[key] = m
		}
	}
}

func (subM subscriptionMap) String() string {
	s := "subscriptionMap{"
	for key, m := range subM {
		s += "\"" + key + "\": ["
		for u := range m {
			s += u.String() + " "
		}
		s = strings.TrimSuffix(s, " ") + "], "
	}
	s = strings.TrimSuffix(s, ", ") + "}"
	return s
}
