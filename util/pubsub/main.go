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
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/util/durationlog"
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

		// drainChanDuration is the max duration during draining channels
		drainChanDuration time.Duration
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

	cmdGetLasts struct {
		id       uuid.UUID
		labels   labelMap
		dataType string
		resp     chan<- []any
	}

	cmdGetLast struct {
		id       uuid.UUID
		labels   labelMap
		dataType string
		resp     chan<- any
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

	cacheEntry struct {
		cmdPub      cmdPub
		publishedAt time.Time
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
		lastPub     map[string]cacheEntry
		started     bool

		// drainChanDuration is the max duration during draining private and exposed
		// channel
		drainChanDuration time.Duration
	}

	stringer interface {
		String() string
	}
)

var (
	cmdDurationWarn    = time.Second
	notifyDurationWarn = 5 * time.Second

	// defaultDrainChanDuration is the default duration to wait while draining channel
	defaultDrainChanDuration = 10 * time.Millisecond
)

// Key returns labelMap key as a string
// with ordered label names
func (t labelMap) Key() string {
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
	b.lastPub = make(map[string]cacheEntry)
	b.log = log.Logger.With().Str("bus", name).Logger()
	b.drainChanDuration = defaultDrainChanDuration
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
				case cmdGetLast:
					b.onGetLastCmd(c)
				case cmdGetLasts:
					b.onGetLastsCmd(c)
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
	b.started = <-started
	b.log.Info().Msg("bus started")
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
	}
	b.subs[id] = sub
	c.resp <- sub
	b.log.Debug().Msgf("subscribe %s timeout %s queueSize %d", sub.name, c.timeout, c.queueSize)
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
	b.log.Debug().Msgf("unsubscribe %s", sub.name)
}

func (b *Bus) onPubCmd(c cmdPub) {
	// store last event to serve subscribers using AddFilterGetLast()
	b.lastPub[c.key()] = cacheEntry{
		cmdPub:      c,
		publishedAt: time.Now(),
	}

	for _, toFilterKey := range c.keys() {
		// search publication that listen on one of cmdPub.keys
		if subIdM, ok := b.subMap[toFilterKey]; ok {
			for subId := range subIdM {
				sub, ok := b.subs[subId]
				if !ok {
					// This should not happen
					b.log.Warn().Msgf("filter key %s has a dead subscription %s", toFilterKey, subId)
					continue
				}
				b.log.Debug().Msgf("route %s to %s", c, sub)
				sub.q <- c.data
			}
		}
	}
	c.resp <- true
}

func (bus *Bus) onGetLastsCmd(c cmdGetLasts) {
	var lasts []any
	filterKey := fmtKey(c.dataType, c.labels)
	for _, entry := range bus.lastPub {
		for _, key := range pubKeys(entry.cmdPub.dataType, entry.cmdPub.labels) {
			if key == filterKey {
				lasts = append(lasts, entry.cmdPub.data)
			}
		}
	}
	c.resp <- lasts
}

func (bus *Bus) onGetLastCmd(c cmdGetLast) {
	var last any
	lastPublished := time.Time{}
	filterKey := fmtKey(c.dataType, c.labels)
	for _, entry := range bus.lastPub {
		for _, key := range pubKeys(entry.cmdPub.dataType, entry.cmdPub.labels) {
			if (key == filterKey) && entry.publishedAt.After(lastPublished) {
				last = entry.cmdPub.data
				lastPublished = entry.publishedAt
			}
		}
	}
	c.resp <- last
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

func (b *Bus) drain() {
	b.log.Info().Msg("draining")
	defer b.log.Info().Msg("drained")
	i := 0
	tC := time.After(b.drainChanDuration)
	for {
		select {
		case <-b.cmdC:
			i += 1
		case <-tC:
			b.log.Info().Msgf("drained dropped %d pending messages from the bus on stop", i)
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

type (
	Timeouter interface {
		timout() time.Duration
	}

	QueueSizer interface {
		queueSize() uint64
	}
)

type (
	QueueSize uint64
	Timeout   time.Duration
)

// queueSize implements QueueSizer for QueueSize
func (t QueueSize) queueSize() uint64 {
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
// default is 2000
func (b *Bus) Sub(name string, options ...interface{}) *Subscription {
	respC := make(chan *Subscription)
	op := cmdSub{
		name:      name,
		resp:      respC,
		queueSize: 2000,
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

func (t cmdPub) key() string {
	return fmtKey(t.dataType, t.labels)
}

func (t filter) key() string {
	return fmtKey(t.dataType, t.labels)
}

func (pub cmdPub) keys() []string {
	return pubKeys(pub.dataType, pub.labels)
}

func fmtKey(dataType string, labels labelMap) string {
	return dataType + ":" + labels.Key()
}

func pubKeys(dataType string, labels labelMap) []string {
	return append(
		keys(dataType, labels),
		keys("", labels)...,
	)
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

func (sub *Subscription) AddFilterGetLasts(v any, labels ...Label) []any {
	sub.AddFilter(v, labels...)
	return sub.GetLasts(v, labels...)
}

// GetLasts returns all last published events of each type and labelset, matching type and labels
func (sub *Subscription) GetLasts(v any, labels ...Label) []any {
	respC := make(chan []any)
	op := cmdGetLasts{
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
		return nil
	}
	select {
	case last := <-respC:
		return last
	case <-sub.bus.ctx.Done():
		return nil
	}
}

func (sub *Subscription) AddFilterGetLast(v any, labels ...Label) any {
	sub.AddFilter(v, labels...)
	return sub.GetLast(v, labels...)
}

// GetLast returns the last published event matching type and labels
func (sub *Subscription) GetLast(v any, labels ...Label) any {
	respC := make(chan any)
	op := cmdGetLast{
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
		return nil
	}
	return <-respC
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
				if err := sub.push(i); err != nil {
					// the subscription got push error, cancel it and ask for unsubscribe
					sub.bus.log.Warn().Msgf("%s error: %s. stop subscription", sub, err)
					go sub.bus.Pub(SubscriptionError{Name: sub.name, Id: sub.id, Error: err})
					sub.cancel()
					go func() {
						if err := sub.Stop(); err != nil {
							sub.bus.log.Warn().Err(err).Msgf("stop %s", sub)
						}
					}()
					sub.bus.endNotify <- sub.id
					return
				}
				sub.bus.endNotify <- sub.id
			}
		}
	}()
	<-started
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
			return errors.New("push exceed timeout " + sub.timeout.String())
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
