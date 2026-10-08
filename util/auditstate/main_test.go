package auditstate

import (
	"sync"
	"testing"

	"github.com/opensvc/om3/v3/util/plog"
)

func begin(r *Registry, user string, preempt bool) (chan plog.LogMessage, chan struct{}, Session, bool) {
	q := make(chan plog.LogMessage)
	preemptC := make(chan struct{})
	sess, ok := r.Begin(q, nil, preemptC, user, preempt)
	return q, preemptC, sess, ok
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
