package evwatch

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
)

// State is the state of a Runner.
type State string

const (
	// StateIdle is a runner with no resource of its kind: its source
	// does not run.
	StateIdle State = "idle"

	// StateRunning is a source running.
	StateRunning State = "running"

	// StateDraining is a source running on with no resource of its kind
	// left, until its grace delay ends.
	StateDraining State = "draining"

	// StateBackoff is a source waiting to run again, once it ended or
	// its check failed.
	StateBackoff State = "backoff"

	// StateDisabled is a source that can not run on the node, until no
	// resource of its kind is left.
	StateDisabled State = "disabled"
)

// Runner runs a source while the resources of its kind are not zero.
//
// A demand going from zero to more starts the source at once: the risk is an
// event missed, not a source started for nothing. A demand back to zero stops
// it once the stop grace of its policy ends, unless the demand comes back
// meanwhile. A source ending runs again after a delay doubled at each quick
// failure, and one that can not run on the node is disabled until the demand
// is back to zero.
//
// One goroutine owns the state: SetDemand never blocks, the latest demand
// wins, and Stop returns once the source stopped.
type Runner struct {
	src     Source
	policy  Policy
	log     *plog.Logger
	onReady func()
	onEvent func(key string)

	demandC chan int
	readyC  chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}

	mu    sync.Mutex
	state State
}

// NewRunner returns a runner of src, calling onReady each time the source is
// ready, for a resync of the resources it misses events of, and onEvent for
// each event.
func NewRunner(src Source, policy Policy, log *plog.Logger, onReady func(), onEvent func(key string)) *Runner {
	return &Runner{
		src:     src,
		policy:  policy,
		log:     log,
		onReady: onReady,
		onEvent: onEvent,
		demandC: make(chan int, 1),
		readyC:  make(chan struct{}, 1),
		state:   StateIdle,
	}
}

// State returns the state of the runner.
func (t *Runner) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

func (t *Runner) setState(state State) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state != state {
		t.log.Debugf("%s: %s -> %s", t.src.Name(), t.state, state)
	}
	t.state = state
}

// Start starts the runner.
func (t *Runner) Start(ctx context.Context) {
	ctx, t.cancel = context.WithCancel(ctx)
	t.done = make(chan struct{})
	go t.loop(ctx)
}

// Stop stops the source, and returns once it stopped.
func (t *Runner) Stop() {
	if t.cancel == nil {
		return
	}
	t.cancel()
	<-t.done
}

// SetDemand says how many resources of its kind the node holds. It never
// blocks: a demand not yet read is replaced.
func (t *Runner) SetDemand(n int) {
	for {
		select {
		case t.demandC <- n:
			return
		default:
		}
		select {
		case <-t.demandC:
		default:
		}
	}
}

func (t *Runner) loop(ctx context.Context) {
	defer close(t.done)
	var (
		demand     int
		runCancel  context.CancelFunc
		runDone    chan error
		runStarted time.Time
		timer      *time.Timer
		timerC     <-chan time.Time
		failures   int
		failing    bool
	)
	stopTimer := func() {
		if timer != nil {
			timer.Stop()
		}
		timer, timerC = nil, nil
	}
	armTimer := func(d time.Duration) {
		stopTimer()
		timer = time.NewTimer(d)
		timerC = timer.C
	}
	stopRun := func() {
		if runCancel == nil {
			return
		}
		runCancel()
		<-runDone
		runCancel, runDone = nil, nil
	}
	defer stopTimer()
	defer stopRun()
	backoff := func() {
		delay := t.policy.RestartMin
		for i := 1; i < failures && delay < t.policy.RestartMax; i++ {
			delay *= 2
		}
		if delay > t.policy.RestartMax {
			delay = t.policy.RestartMax
		}
		t.setState(StateBackoff)
		armTimer(delay)
	}
	attempt := func() {
		checkCtx, cancel := context.WithTimeout(ctx, t.policy.CheckTimeout)
		err := t.src.Check(checkCtx)
		cancel()
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, ErrUnsupported):
			t.log.Infof("%s: disabled: %s", t.src.Name(), err)
			t.setState(StateDisabled)
			return
		case err != nil:
			failures++
			t.logFailure(&failing, "check", err)
			backoff()
			return
		}
		runCtx, cancelRun := context.WithCancel(ctx)
		done := make(chan error, 1)
		ready := func() {
			if t.onReady != nil {
				t.onReady()
			}
			select {
			case t.readyC <- struct{}{}:
			default:
			}
		}
		emit := func(key string) {
			if t.onEvent != nil {
				t.onEvent(key)
			}
		}
		go func() {
			done <- t.src.Run(runCtx, ready, emit)
		}()
		runCancel, runDone, runStarted = cancelRun, done, time.Now()
		t.setState(StateRunning)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case n := <-t.demandC:
			demand = n
			switch state := t.State(); {
			case demand > 0 && state == StateIdle:
				attempt()
			case demand > 0 && state == StateDraining:
				stopTimer()
				t.setState(StateRunning)
			case demand == 0 && state == StateRunning:
				t.setState(StateDraining)
				armTimer(t.policy.StopGrace)
			case demand == 0 && state == StateBackoff:
				stopTimer()
				failures = 0
				t.setState(StateIdle)
			case demand == 0 && state == StateDisabled:
				failures = 0
				t.setState(StateIdle)
			}
		case <-t.readyC:
			if failing {
				t.log.Infof("%s: watching events again", t.src.Name())
				failing = false
			}
		case err := <-runDone:
			runCancel, runDone = nil, nil
			if ctx.Err() != nil {
				return
			}
			if t.State() == StateDraining || demand == 0 {
				stopTimer()
				failures = 0
				t.setState(StateIdle)
				continue
			}
			if time.Since(runStarted) >= t.policy.QuickFailure {
				failures = 0
			}
			failures++
			t.logFailure(&failing, "ended", err)
			if t.policy.MaxQuickFailures > 0 && failures >= t.policy.MaxQuickFailures {
				t.log.Warnf("%s: disabled: ended %d times in a row right after it started: the changes are seen at the scheduled status only", t.src.Name(), failures)
				t.setState(StateDisabled)
				continue
			}
			backoff()
		case <-timerC:
			timer, timerC = nil, nil
			switch t.State() {
			case StateDraining:
				stopRun()
				failures = 0
				t.setState(StateIdle)
			case StateBackoff:
				if demand > 0 {
					attempt()
				} else {
					t.setState(StateIdle)
				}
			}
		}
	}
}

// logFailure logs the first failure of a series as a warning, the others at
// debug level: a service down for long would flood the log otherwise.
func (t *Runner) logFailure(failing *bool, what string, err error) {
	if *failing {
		t.log.Debugf("%s: %s: %v", t.src.Name(), what, err)
		return
	}
	*failing = true
	t.log.Warnf("%s: %s: %v: retrying, the changes are seen at the scheduled status meanwhile", t.src.Name(), what, err)
}
