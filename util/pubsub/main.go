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

	// subscriptions is a hash of subscription indexed by multiple lookup critera
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

// StartBus allocate and runs a new Bus and return a pointer
func NewBus(name string) *Bus {
	b := &Bus{}
	b.name = name
	b.cmdC = make(chan any)
	b.beginNotify = make(chan uuid.UUID)
	b.endNotify = make(chan uuid.UUID)
	b.log = log.Logger.With().Str("bus", name).Logger()
	return b
}

func (t *Bus) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	started := make(chan bool)
	t.drain() // flush cmds queued while we were stopped ?
	t.Add(1)
	go func() {
		defer t.Done()

		watchDuration := &durationlog.T{Log: t.log}
		watchDurationCtx, watchDurationCancel := context.WithCancel(context.Background())
		defer watchDurationCancel()
		var beginCmd = make(chan any)
		var endCmd = make(chan bool)
		t.Add(1)
		go func() {
			defer t.Done()
			watchDuration.WarnExceeded(watchDurationCtx, beginCmd, endCmd, cmdDurationWarn, "msg")
		}()

		t.Add(1)
		go func() {
			defer t.Done()
			t.warnExceededNotification(watchDurationCtx, notifyDurationWarn)
		}()

		t.subs = make(map[uuid.UUID]*Subscription)
		t.subMap = make(subscriptionMap)

		started <- true
		for {
			select {
			case <-t.ctx.Done():
				return
			case cmd := <-t.cmdC:
				beginCmd <- cmd
				switch c := cmd.(type) {
				case cmdPub:
					t.onPubCmd(c)
				case cmdSubAddFilter:
					t.onSubAddFilter(c)
				case cmdSub:
					id := uuid.New()
					sub := &Subscription{
						name:    c.name,
						C:       make(chan any, notifyQueueSizePerSubscriber),
						q:       make(chan any, notifyQueueSizePerSubscriber),
						id:      id,
						timeout: c.timeout,
						bus:     t,
					}
					t.subs[id] = sub
					t.subMap.Add(id, ":") // listen all until AddFilter is called
					c.resp <- sub
					t.log.Debug().Msgf("subscribe %s", sub.name)
				case cmdUnsub:
					sub, ok := t.subs[c.id]
					if !ok {
						break
					}
					sub.cancel()
					delete(t.subs, c.id)
					t.subMap.Del(c.id, sub.keys()...)
					select {
					case <-t.ctx.Done():
					case c.resp <- sub.name:
					}
					t.log.Debug().Msgf("unsubscribe %s", sub.name)
				}
				endCmd <- true
			}
		}
	}()
	<-started
	t.log.Info().Msg("bus started")
}

func (t *Bus) onPubCmd(c cmdPub) {
	for _, key := range c.keys() {
		if ids, ok := t.subMap[key]; ok {
			for id, _ := range ids {
				sub := t.subs[id]
				t.log.Debug().Msgf("route %s to %s", c, sub)
				sub.q <- c.data
			}
		}
	}
	select {
	case <-t.ctx.Done():
	case c.resp <- true:
	}
}

func (t *Bus) onSubAddFilter(c cmdSubAddFilter) {
	sub, ok := t.subs[c.id]
	if !ok {
		c.resp <- nil
		return
	}
	sub.filters = append(sub.filters, filter{
		dataType: c.dataType,
		labels:   c.labels,
	})
	t.subs[c.id] = sub
	t.subMap.Del(c.id, ":")
	t.subMap.Add(c.id, sub.keys()...)
	c.resp <- nil
}

func (t *Bus) drain() {
	i := 0
	defer func() {
		if i > 0 {
			t.log.Info().Msg("dropped %d pending messages from the bus on stop")
		}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-t.cmdC:
			i += 1
		case <-ticker.C:
			return
		}
	}
}

func (t *Bus) Stop() {
	if t == nil {
		return
	}
	if t.cancel != nil {
		f := t.cancel
		t.cancel = nil
		f()
		t.Wait()
		t.log.Info().Msg("stopped")
		t.drain()
	}
}

// Pub posts a new Publication on the bus
func (t Bus) Pub(v any, labels ...Label) {
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
	case t.cmdC <- op:
	case <-t.ctx.Done():
		return
	}
	select {
	case <-done:
		return
	case <-t.ctx.Done():
		return
	}
}

// Sub function requires a new Subscription on the bus.
func (t Bus) Sub(name string) *Subscription {
	return t.SubWithTimeout(name, 0*time.Second)
}

// SubWithTimeout function requires a new Subscription on the bus.
// Enforce a timeout for the subscriber to pull each message.
func (t Bus) SubWithTimeout(name string, timeout time.Duration) *Subscription {
	respC := make(chan *Subscription)
	op := cmdSub{
		name:    name,
		resp:    respC,
		timeout: timeout,
	}
	select {
	case t.cmdC <- op:
	case <-t.ctx.Done():
		return nil
	}
	select {
	case as := <-respC:
		return as
	case <-t.ctx.Done():
		return nil
	}
}

// Unsub function remove a subscription
func (t Bus) unsub(sub Subscription) string {
	respC := make(chan string)
	op := cmdUnsub{
		id:   sub.id,
		resp: respC,
	}
	select {
	case t.cmdC <- op:
	case <-t.ctx.Done():
		return ""
	}
	select {
	case s := <-respC:
		return s
	case <-t.ctx.Done():
		return ""
	}
}

// warnExceededNotification log when notify duration between <-begin and <-end exceeds maxDuration.
func (t Bus) warnExceededNotification(ctx context.Context, maxDuration time.Duration) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	pending := make(map[uuid.UUID]time.Time)
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-t.beginNotify:
			pending[id] = time.Now()
		case id := <-t.endNotify:
			delete(pending, id)
		case <-ticker.C:
			now := time.Now()
			for id, begin := range pending {
				if now.Sub(begin) > maxDuration {
					duration := time.Now().Sub(begin).Seconds()
					sub := t.subs[id]
					t.log.Warn().Msgf("waited %.02fs over %s for %s", duration, maxDuration, sub)
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

func (t cmdPub) keys() []string {
	return append(
		keys(t.dataType, t.labels),
		keys("", t.labels)...,
	)
}

func (t cmdPub) String() string {
	var dataS string
	switch data := t.data.(type) {
	case stringer:
		dataS = data.String()
	default:
		dataS = "type " + t.dataType
	}
	s := fmt.Sprintf("publication %s", dataS)
	if len(t.labels) > 0 {
		s += " with " + t.labels.String()
	}
	return s
}

func (t cmdSubAddFilter) String() string {
	s := fmt.Sprintf("add subscription %s filter type %s", t.id, t.dataType)
	if len(t.labels) > 0 {
		s += " with " + t.labels.String()
	}
	return s
}

func (t cmdSub) String() string {
	s := fmt.Sprintf("subscribe '%s'", t.name)
	return s
}

func (t cmdUnsub) String() string {
	return fmt.Sprintf("unsubscribe key %s", t.id)
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
func (t Subscription) drain() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.q:
		case <-ticker.C:
			return
		}
	}
}

func (t Subscription) keys() []string {
	if len(t.filters) == 0 {
		return []string{":"}
	}
	l := make([]string, 0)
	for _, f := range t.filters {
		l = append(l, f.key())
	}
	return l
}

func (t filter) key() string {
	return t.dataType + ":" + t.labels.Key()
}

func (t Subscription) String() string {
	s := fmt.Sprintf("subscription '%s'", t.name)
	for _, f := range t.filters {
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

func (t Subscription) AddFilter(v any, labels ...Label) {
	respC := make(chan error)
	op := cmdSubAddFilter{
		id:     t.id,
		labels: newLabels(labels...),
		resp:   respC,
	}
	dataType := reflect.TypeOf(v)
	if dataType != nil {
		op.dataType = dataType.String()
	}
	select {
	case t.bus.cmdC <- op:
	case <-t.bus.ctx.Done():
		return
	}
	select {
	case <-respC:
		return
	case <-t.bus.ctx.Done():
		return
	}
}

func (t *Subscription) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	started := make(chan bool)
	t.bus.Add(1)
	go func() {
		t.bus.Done()
		defer t.cancel()
		defer t.drain()
		started <- true
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.bus.ctx.Done():
				return
			case i := <-t.q:
				t.bus.beginNotify <- t.id
				startTime := time.Now()
				if t.push(i) != nil {
					// the subscription is too slow, kill it
					// then ask for unsubscribe
					duration := time.Now().Sub(startTime).Seconds()
					t.bus.log.Warn().Msgf("waited %.02fs for %s => stop subscription", duration, t)
					t.cancel()
					go t.Stop()
					t.bus.endNotify <- t.id
					return
				}
				t.bus.endNotify <- t.id
			}
		}
	}()
	<-started
}

func (t Subscription) Stop() string {
	return t.bus.unsub(t)
}

func (t Subscription) push(i any) error {
	if t.timeout == 0 {
		t.C <- i
	} else {
		timer := time.NewTimer(t.timeout)
		select {
		case t.C <- i:
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

func (t subscriptionMap) Del(id uuid.UUID, keys ...string) {
	for _, key := range keys {
		if m, ok := t[key]; ok {
			delete(m, id)
			t[key] = m
		}
	}
}

func (t subscriptionMap) Add(id uuid.UUID, keys ...string) {
	for _, key := range keys {
		if m, ok := t[key]; ok {
			m[id] = nil
			t[key] = m
		} else {
			m = map[uuid.UUID]any{id: nil}
			t[key] = m
		}
	}
}

func (t subscriptionMap) String() string {
	s := "subscriptionMap{"
	for key, m := range t {
		s += "\"" + key + "\": ["
		for u, _ := range m {
			s += u.String() + " "
		}
		s = strings.TrimSuffix(s, " ") + "], "
	}
	s = strings.TrimSuffix(s, ", ") + "}"
	return s
}
