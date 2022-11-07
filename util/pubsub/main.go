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
	// Labels allow message routing filtering based on key/value matching
	Labels map[string]string

	// Label is a {key, val} array
	Label [2]string

	// subscriptions is a hash of subscription indexed by multiple lookup critera
	subscriptionMap map[string]map[uuid.UUID]any

	Subscription struct {
		labels   Labels
		dataType string
		name     string
		id       uuid.UUID
		bus      *Bus

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
		labels   Labels
		dataType string
		data     any
		resp     chan<- bool
	}

	cmdSub struct {
		labels   Labels
		dataType string
		name     string
		resp     chan<- Subscription
		timeout  time.Duration
	}

	cmdUnsub struct {
		id   uuid.UUID
		resp chan<- string
	}

	Bus struct {
		sync.WaitGroup
		name   string
		cmdC   chan any
		cancel func()
		log    zerolog.Logger
		ctx    context.Context
		subs   map[uuid.UUID]Subscription
		subMap subscriptionMap
	}

	stringer interface {
		String() string
	}
)

var (
	cmdDurationWarn    = time.Second
	notifyDurationWarn = 5 * time.Second
)

func (t Labels) Key() string {
	s := ""
	for _, key := range xmap.Keys(t) {
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

func newLabels(labels ...Label) Labels {
	m := make(Labels)
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
	b.log = log.Logger.With().Str("bus", name).Logger()
	return b
}

func (t *Bus) Start(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	started := make(chan bool)
	t.Empty() // flush cmds queued while we were stopped ?
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

		var beginNotify = make(chan uuid.UUID)
		var endNotify = make(chan uuid.UUID)
		t.Add(1)
		go func() {
			defer t.Done()
			t.warnExceededNotification(watchDurationCtx, beginNotify, endNotify, notifyDurationWarn)
		}()

		t.subs = make(map[uuid.UUID]Subscription)
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
					for _, key := range c.Keys() {
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
				case cmdSub:
					id := uuid.New()
					subCtx, subCtxCancel := context.WithCancel(context.Background())
					sub := Subscription{
						labels:   c.labels,
						name:     c.name,
						dataType: c.dataType,
						C:        make(chan any, notifyQueueSizePerSubscriber),
						q:        make(chan any, notifyQueueSizePerSubscriber),
						id:       id,
						timeout:  c.timeout,
						cancel:   subCtxCancel,
						bus:      t,
					}
					key := sub.Key()
					t.subs[id] = sub
					t.subMap.Add(id, key)
					started := make(chan bool)
					t.Add(1)
					go func() {
						t.Done()
						defer sub.cancel()
						defer func() {
							// empty any pending message for this subscription
							ticker := time.NewTicker(2 * time.Second)
							defer ticker.Stop()
							for {
								select {
								case <-sub.q:
								case <-ticker.C:
									return
								}
							}
						}()
						started <- true
						for {
							select {
							case <-subCtx.Done():
								return
							case i := <-sub.q:
								beginNotify <- id
								startTime := time.Now()
								if sub.push(i) != nil {
									// the subscription is too slow, kill it
									// then ask for unsubscribe
									duration := time.Now().Sub(startTime).Seconds()
									t.log.Warn().Msgf("waited %.02fs for %s => stop subscription", duration, sub)
									sub.cancel()
									go sub.Stop()
									endNotify <- id
									return
								}
								endNotify <- id
							case <-t.ctx.Done():
								return
							}
						}
					}()
					<-started
					c.resp <- sub
					t.log.Debug().Msgf("subscribe %s", sub.name)
				case cmdUnsub:
					sub, ok := t.subs[c.id]
					if !ok {
						break
					}
					sub.cancel()
					delete(t.subs, c.id)
					t.subMap.Del(c.id, sub.Key())
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

func (t *Bus) Empty() {
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
		t.Empty()
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

// SubWithTimeout function requires a new Subscription on the bus.
func (t Bus) Sub(name string, v any, labels ...Label) Subscription {
	return t.SubWithTimeout(name, v, 0*time.Second, labels...)
}

// SubWithTimeout function requires a new Subscription on the bus.
// Enforce a timeout for the subscriber to pull each message.
func (t Bus) SubWithTimeout(name string, v any, timeout time.Duration, labels ...Label) Subscription {
	respC := make(chan Subscription)
	op := cmdSub{
		labels:  newLabels(labels...),
		name:    name,
		resp:    respC,
		timeout: timeout,
	}
	dataType := reflect.TypeOf(v)
	if dataType != nil {
		op.dataType = dataType.String()
	}
	select {
	case t.cmdC <- op:
	case <-t.ctx.Done():
		return Subscription{}
	}
	select {
	case as := <-respC:
		return as
	case <-t.ctx.Done():
		return Subscription{}
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
func (t Bus) warnExceededNotification(ctx context.Context, begin <-chan uuid.UUID, end <-chan uuid.UUID, maxDuration time.Duration) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	pending := make(map[uuid.UUID]time.Time)
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-begin:
			pending[id] = time.Now()
		case id := <-end:
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

func (t cmdPub) Keys() []string {
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

func (t cmdSub) String() string {
	s := fmt.Sprintf("subscribe '%s' on type %s", t.name, t.dataType)
	if len(t.labels) > 0 {
		s += " with " + t.labels.String()
	}
	return s
}

func (t cmdUnsub) String() string {
	return fmt.Sprintf("unsubscribe key %s", t.id)
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

func (t Subscription) Key() string {
	return t.dataType + ":" + t.labels.Key()
}

func (t Subscription) String() string {
	s := fmt.Sprintf("subscription '%s'", t.name)
	if t.dataType != "" {
		s += " on msg type " + t.dataType
	} else {
		s += " on msg type *"
	}
	if len(t.labels) > 0 {
		s += " with " + t.labels.String()
	}
	return s
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

func (t subscriptionMap) Del(id uuid.UUID, key string) {
	if m, ok := t[key]; ok {
		delete(m, id)
		t[key] = m
	}
}

func (t subscriptionMap) Add(id uuid.UUID, key string) {
	if m, ok := t[key]; ok {
		m[id] = nil
		t[key] = m
	} else {
		m = map[uuid.UUID]any{id: nil}
		t[key] = m
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
