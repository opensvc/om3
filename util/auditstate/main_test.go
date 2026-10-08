package auditstate

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
)

func begin(r *Registry, user string, preempt bool) (chan plog.LogMessage, chan struct{}, Session, bool) {
	q := make(chan plog.LogMessage)
	preemptC := make(chan struct{})
	sess, err := r.Begin(context.Background(), q, nil, preemptC, user, preempt, nil)
	return q, preemptC, sess, err == nil
}

func TestBeginRefusesASecondSession(t *testing.T) {
	r := &Registry{}
	q1, _, _, ok := begin(r, "u1", false)
	if !ok {
		t.Fatal("the first session is refused")
	}
	_, _, sess, ok := begin(r, "u2", false)
	if ok {
		t.Fatal("a second session begins without preempt")
	}
	if sess.User != "u1" {
		t.Fatalf("the refusal names the session of %q, want u1", sess.User)
	}
	if cur, _ := r.Snapshot(); cur.Q != q1 {
		t.Fatal("the refused session replaced the current one")
	}
}

func TestBeginPreempts(t *testing.T) {
	r := &Registry{}
	q1, preempt1, _, _ := begin(r, "u1", false)
	q2, preempt2, _, ok := begin(r, "u2", true)
	if !ok {
		t.Fatal("a preempting session is refused")
	}
	select {
	case <-preempt1:
	default:
		t.Fatal("the preempted session is not told")
	}
	select {
	case <-preempt2:
		t.Fatal("the preempting session is told it is preempted")
	default:
	}

	// The preempted session stops after the preempting one began: that
	// stop must not end the current session.
	r.Stop(q1)
	if cur, ok := r.Snapshot(); !ok || cur.Q != q2 {
		t.Fatal("the stop of the preempted session ended the current one")
	}
	r.Stop(q2)
	if r.Active() {
		t.Fatal("a session is active after the last stop")
	}
}

func TestBeginConcurrentAtMostOne(t *testing.T) {
	r := &Registry{}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		begun int
	)
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, _, ok := begin(r, "u", false); ok {
				mu.Lock()
				begun++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if begun != 1 {
		t.Fatalf("%d sessions began at once, want 1", begun)
	}
}

// TestBeginActivatesInOrder preempts a session between its registration and
// its activation: the session preempting it must activate last, so the
// subsystems send their logs to it.
func TestBeginActivatesInOrder(t *testing.T) {
	r := &Registry{}
	var (
		mu    sync.Mutex
		order []string
		done  = make(chan struct{})
	)
	activated := func(name string) {
		mu.Lock()
		order = append(order, name)
		mu.Unlock()
	}
	q1 := make(chan plog.LogMessage)
	_, err := r.Begin(context.Background(), q1, nil, make(chan struct{}), "u1", false, func() {
		go func() {
			defer close(done)
			r.Begin(context.Background(), make(chan plog.LogMessage), nil, make(chan struct{}), "u2", true, func() {
				activated("preempting")
			})
		}()
		// Give the preempting session the time to activate first, as it
		// could without the ordering.
		time.Sleep(50 * time.Millisecond)
		activated("preempted")
	})
	if err != nil {
		t.Fatal("the first session is refused")
	}
	<-done
	if len(order) != 2 || order[0] != "preempted" || order[1] != "preempting" {
		t.Fatalf("activation order %v, want [preempted preempting]", order)
	}
}

// TestBeginGivesUp begins a session whose activation does not return, as when
// the bus does not accept its publication: the next session, preempting or
// not, gives up when its context is done, rather than wait for ever.
func TestBeginGivesUp(t *testing.T) {
	r := &Registry{}
	activating := make(chan struct{})
	unblock := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Begin(context.Background(), make(chan plog.LogMessage), nil, make(chan struct{}), "u1", false, func() {
			close(activating)
			<-unblock
		})
	}()
	<-activating

	for _, preempt := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_, err := r.Begin(ctx, make(chan plog.LogMessage), nil, make(chan struct{}), "u2", preempt, nil)
		cancel()
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("preempt %v: err %v, want ErrBusy", preempt, err)
		}
	}

	// The slot is free again once the activation returns.
	close(unblock)
	<-done
	_, err := r.Begin(context.Background(), make(chan plog.LogMessage), nil, make(chan struct{}), "u2", true, nil)
	if err != nil {
		t.Fatalf("a preempting session is refused after the activation returned: %v", err)
	}
}

func TestBeginActiveError(t *testing.T) {
	r := &Registry{}
	begin(r, "u1", false)
	sess, err := r.Begin(context.Background(), make(chan plog.LogMessage), nil, make(chan struct{}), "u2", false, nil)
	if !errors.Is(err, ErrActive) {
		t.Fatalf("err %v, want ErrActive", err)
	}
	if sess.User != "u1" {
		t.Fatalf("the refusal names the session of %q, want u1", sess.User)
	}
}
