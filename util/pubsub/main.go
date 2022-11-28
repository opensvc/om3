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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/util/durationlog"
	"opensvc.com/opensvc/util/stringslice"
	"opensvc.com/opensvc/util/xmap"
)

type (
	contextKey int
)

const (
	busContextKey contextKey = 0

	// notifyQueueSizePerSubscriber defines notify max queue size for a subscriber
	notifyQueueSizePerSubscriber = 2000
)

type (
	// labelMap allow message routing filtering based on key/value matching
	labelMap map[string]string

	// Label is a {key, val} array
	Label [2]string

	// subscriptions is a hash of subscription indexed by multiple lookup criteria
	subscriptionMap map[string]map[uuid.UUID]any

	filter struct {
		labels   labelMap
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
	}

	cmdPub struct {
		labels   labelMap
		dataType string
		data     any
		resp     chan<- bool
	}

	cmdSubAddFilter struct {
		id       uuid.UUID
		labels   labelMap
		dataType string
		resp     chan<- error
	}

	cmdSub struct {
		name    string
		resp    chan<- *Subscription
		timeout time.Duration
	}

	cmdUnsub struct {
		id   uuid.UUID
		resp chan<- string
	}

	Bus struct {
		sync.WaitGroup
		name        string
		cmdC        chan any
		cancel      func()
		log         zerolog.Logger
		ctx         context.Context
		subs        map[uuid.UUID]*Subscription
		subMap      subscriptionMap
		beginNotify chan uuid.UUID
		endNotify   chan uuid.UUID
	}

	stringer interface {
		String() string
	}
)

var (
	cmdDurationWarn    = time.Second
	notifyDurationWarn = 5 * time.Second
)

func (t labelMap) Key() string {
	s := ""
	for _, key := range xmap.Keys(t) {
		s += "{" + key + "=" + t[key] + "}"
	}
	return s
}

// keys returns all the permutations of all lengths of the labels
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
func (t labelMap) keys() []string {
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

func newLabels(labels ...Label) labelMap {
	m := make(labelMap)
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
	b.log = log.Logger.With().Str("bus", name).Logger()
	return b
}

func (b *Bus) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	started := make(chan bool)
	b.subs = make(map[uuid.UUID]*Subscription)
	b.subMap = make(subscriptionMap)

	b.Add(1)
	go func() {
		defer b.Done()

		watchDuration := &durationlog.T{Log: b.log}
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
				case cmdSub:
					b.onSubCmd(c)
				case cmdUnsub:
					b.onUnsubCmd(c)
				}
				endCmd <- true
			}
		}
	}()
	<-started
	b.log.Info().Msg("bus started")
}

func (b *Bus) onSubCmd(c cmdSub) {
	id := uuid.New()
	sub := &Subscription{
		name:    c.name,
		C:       make(chan any, notifyQueueSizePerSubscriber),
		q:       make(chan any, notifyQueueSizePerSubscriber),
		id:      id,
		timeout: c.timeout,
		bus:     b,
	}
	b.subs[id] = sub
	c.resp <- sub
	b.log.Debug().Msgf("subscribe %s", sub.name)
}

func (b *Bus) onUnsubCmd(c cmdUnsub) {
	sub, ok := b.subs[c.id]
	if !ok {
		return
	}
	sub.cancel()
	delete(b.subs, c.id)
	b.subMap.Del(c.id, sub.keys()...)
	select {
	case <-b.ctx.Done():
	case c.resp <- sub.name:
	}
	b.log.Debug().Msgf("unsubscribe %s", sub.name)
}

func (b *Bus) onPubCmd(c cmdPub) {
	for _, key := range c.keys() {
		if ids, ok := b.subMap[key]; ok {
			for id := range ids {
				sub := b.subs[id]
				b.log.Debug().Msgf("route %s to %s", c, sub)
				sub.q <- c.data
			}
		}
	}
	select {
	case <-b.ctx.Done():
	case c.resp <- true:
	}
}

func (b *Bus) onSubAddFilter(c cmdSubAddFilter) {
	sub, ok := b.subs[c.id]
	if !ok {
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

func (b *Bus) drain() {
	b.log.Debug().Msg("draining")
	defer b.log.Debug().Msg("drained")
	i := 0
	tC := time.After(100 * time.Millisecond)
	for {
		select {
		case <-tC:
			b.log.Debug().Msgf("drained dropped %d pending messages from the bus on stop", i)
		case <-b.cmdC:
			i += 1
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
		b.log.Info().Msg("stopped")
	}
}

// Pub posts a new Publication on the bus
func (b *Bus) Pub(v any, labels ...Label) {
	done := make(chan bool)
	op := cmdPub{
		labels: newLabels(labels...),
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
	select {
	case <-done:
		return
	case <-b.ctx.Done():
		return
	}
}

// Sub function requires a new Subscription on the bus.
func (b *Bus) Sub(name string) *Subscription {
	return b.SubWithTimeout(name, 0*time.Second)
}

// SubWithTimeout function requires a new Subscription on the bus.
// Enforce a timeout for the subscriber to pull each message.
func (b *Bus) SubWithTimeout(name string, timeout time.Duration) *Subscription {
	respC := make(chan *Subscription)
	op := cmdSub{
		name:    name,
		resp:    respC,
		timeout: timeout,
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return nil
	}
	select {
	case as := <-respC:
		return as
	case <-b.ctx.Done():
		return nil
	}
}

// Unsub function remove a subscription
func (b *Bus) unsub(sub *Subscription) string {
	respC := make(chan string)
	op := cmdUnsub{
		id:   sub.id,
		resp: respC,
	}
	select {
	case b.cmdC <- op:
	case <-b.ctx.Done():
		return ""
	}
	select {
	case s := <-respC:
		return s
	case <-b.ctx.Done():
		return ""
	}
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
					b.log.Warn().Msgf("waited %.02fs over %s for %s", duration, maxDuration, sub)
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

func (pub cmdPub) keys() []string {
	return append(
		keys(pub.dataType, pub.labels),
		keys("", pub.labels)...,
	)
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

func (t labelMap) String() string {
	if len(t) == 0 {
		return ""
	}
	s := "labels"
	for k, v := range t {
		s += fmt.Sprintf(" %s=%s", k, v)
	}
	return s
}

// drain dequeues any pending message
func (sub *Subscription) drain() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-sub.q:
		case <-ticker.C:
			return
		}
	}
}

func (sub *Subscription) keys() []string {
	if len(sub.filters) == 0 {
		return []string{":"}
	}
	l := make([]string, 0)
	for _, f := range sub.filters {
		l = append(l, f.key())
	}
	return l
}

func (t filter) key() string {
	return t.dataType + ":" + t.labels.Key()
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
	select {
	case <-respC:
		return
	case <-sub.bus.ctx.Done():
		return
	}
}

func (sub *Subscription) Start() {
	if len(sub.filters) == 0 {
		// listen all until AddFilter is called
		sub.AddFilter(nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
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
			case <-sub.bus.ctx.Done():
				return
			case i := <-sub.q:
				sub.bus.beginNotify <- sub.id
				startTime := time.Now()
				if sub.push(i) != nil {
					// the subscription is too slow, kill it
					// then ask for unsubscribe
					duration := time.Now().Sub(startTime).Seconds()
					sub.bus.log.Warn().Msgf("waited %.02fs for %s => stop subscription", duration, sub)
					sub.cancel()
					go sub.Stop()
					sub.bus.endNotify <- sub.id
					return
				}
				sub.bus.endNotify <- sub.id
			}
		}
	}()
	<-started
}

func (sub *Subscription) Stop() string {
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
			return errors.New("timeout")
		}
	}
	return nil
}

func keys(dataType string, labels labelMap) []string {
	var l []string
	if len(labels) == 0 {
		return []string{dataType + ":"}
	}
	for _, key := range labels.keys() {
		l = append(l, dataType+":"+key)
	}
	return l
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
