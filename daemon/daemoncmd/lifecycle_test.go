package daemoncmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// fakeManager is a systemd whose jobs end with the results it is given.
	fakeManager struct {
		mu                sync.Mutex
		calls             []string
		state             string
		calledFromManager bool
		startResult       string
		stopResult        string
		restartResult     string

		// restartJob, when set, holds the restart job until a result is
		// sent on it.
		restartJob chan string

		// onStop runs when the stop job is done, as the stop of the
		// daemon of the unit.
		onStop func()

		// onStart runs when the start job is done, as the start of the
		// daemon of the unit.
		onStart func()
	}

	// fakeAPI is the api of a daemon.
	fakeAPI struct {
		mu    sync.Mutex
		posts int

		// answers says the api answers.
		answers bool

		// postStop serves a stop request.
		postStop func() (int, error)
	}

	// fakeProcesses are the daemon processes, and the pid file.
	fakeProcesses struct {
		mu      sync.Mutex
		pidFile int
		alive   map[int]bool
		signals []string

		// notDaemon are the processes whose command line is not the one
		// of a daemon.
		notDaemon map[int]bool

		// onSignal runs on a signal, as the effect of the signal.
		onSignal func(pid int, sig syscall.Signal)
	}
)

func jobError(op, result string) error {
	if result == "" || result == "done" {
		return nil
	}
	return fmt.Errorf("%s opensvc-server.service: job %s", op, result)
}

func (m *fakeManager) record(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, s)
}

func (m *fakeManager) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.calls...)
}

func (m *fakeManager) ActiveState(_ context.Context) (string, error) {
	m.record("ActiveState")
	return m.state, nil
}

func (m *fakeManager) CalledFromManager() bool { return m.calledFromManager }

func (m *fakeManager) Close() error { return nil }

func (m *fakeManager) Defined(_ context.Context) (bool, error) { return true, nil }

func (m *fakeManager) Start(_ context.Context) error {
	m.record("Start")
	if err := jobError("start", m.startResult); err != nil {
		return err
	}
	if m.onStart != nil {
		m.onStart()
	}
	return nil
}

func (m *fakeManager) Restart(_ context.Context) error {
	m.record("Restart")
	if m.restartJob != nil {
		return jobError("restart", <-m.restartJob)
	}
	return jobError("restart", m.restartResult)
}

func (m *fakeManager) RestartDetached() error {
	m.record("RestartDetached")
	return nil
}

func (m *fakeManager) Stop(_ context.Context) error {
	m.record("Stop")
	if err := jobError("stop", m.stopResult); err != nil {
		return err
	}
	if m.onStop != nil {
		m.onStop()
	}
	return nil
}

func (m *fakeManager) IsSystemStopping() (bool, error) { return false, nil }

func (a *fakeAPI) Ping(_ context.Context) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.answers
}

func (a *fakeAPI) PostStop(_ context.Context) (int, error) {
	a.mu.Lock()
	a.posts++
	f := a.postStop
	a.mu.Unlock()
	if f == nil {
		return -1, errors.New("dial unix http.sock: connect: no such file or directory")
	}
	return f()
}

func (a *fakeAPI) Posts() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.posts
}

func (a *fakeAPI) setAnswers(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.answers = v
}

func (p *fakeProcesses) DaemonPid() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pidFile > 0 && p.alive[p.pidFile] && !p.notDaemon[p.pidFile] {
		return p.pidFile, nil
	}
	return -1, nil
}

func (p *fakeProcesses) PidFileNames(pid int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pidFile == pid
}

func (p *fakeProcesses) Alive(pid int) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive[pid], nil
}

func (p *fakeProcesses) Signal(pid int, sig syscall.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, fmt.Sprintf("%s %d", sig, pid))
	f := p.onSignal
	p.mu.Unlock()
	if f != nil {
		f(pid, sig)
	}
	return nil
}

func (p *fakeProcesses) Signals() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.signals...)
}

// exit is the exit of the daemon of pid: it removes its pid file.
func (p *fakeProcesses) exit(pid int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alive[pid] = false
	if p.pidFile == pid {
		p.pidFile = 0
	}
}

const daemonPid = 4242

// newRunningDaemon returns the api and the processes of a daemon running as
// daemonPid, which a stop request stops when stops is true.
func newRunningDaemon(stops bool) (*fakeAPI, *fakeProcesses) {
	procs := &fakeProcesses{pidFile: daemonPid, alive: map[int]bool{daemonPid: true}}
	api := &fakeAPI{answers: true}
	api.postStop = func() (int, error) {
		if stops {
			procs.exit(daemonPid)
			api.setAnswers(false)
		}
		return daemonPid, nil
	}
	return api, procs
}

// setupLifecycle makes the locks and the waits of the test ones.
func setupLifecycle(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	saved := []any{lockPath, cliLockPath, WaitStoppedTimeout, WaitStoppedDelay, WaitKilledTimeout, WaitRunningTimeout, WaitRunningDelay}
	lockPath = filepath.Join(dir, "main")
	cliLockPath = lockPath + "-cli"
	WaitStoppedTimeout = 100 * time.Millisecond
	WaitStoppedDelay = 5 * time.Millisecond
	WaitKilledTimeout = 50 * time.Millisecond
	WaitRunningTimeout = 100 * time.Millisecond
	WaitRunningDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		lockPath = saved[0].(string)
		cliLockPath = saved[1].(string)
		WaitStoppedTimeout = saved[2].(time.Duration)
		WaitStoppedDelay = saved[3].(time.Duration)
		WaitKilledTimeout = saved[4].(time.Duration)
		WaitRunningTimeout = saved[5].(time.Duration)
		WaitRunningDelay = saved[6].(time.Duration)
	})
}

// A stop under systemd is a stop job of the unit, whatever its state, so it
// replaces a start or a restart: the stop wins. The daemon is stopped through
// its api only when the unit was not running it, as a daemon started outside
// of the unit.
func TestStopWithManagerRoutesPerActiveState(t *testing.T) {
	setupLifecycle(t)
	for _, state := range []string{"active", "activating", "deactivating", "reloading"} {
		t.Run(state, func(t *testing.T) {
			api, procs := newRunningDaemon(true)
			mgr := &fakeManager{state: state, onStop: func() { procs.exit(daemonPid); api.setAnswers(false) }}
			d := &T{daemonsys: mgr, api: api, procs: procs}
			require.NoError(t, d.Stop(context.Background()))
			assert.Equal(t, []string{"ActiveState", "Stop"}, mgr.Calls())
			assert.Equal(t, 0, api.Posts(), "the daemon of the unit is stopped by systemd")
			assert.Empty(t, procs.Signals())
		})
	}
	for _, state := range []string{"inactive", "failed"} {
		t.Run(state+" with a daemon outside of the unit", func(t *testing.T) {
			api, procs := newRunningDaemon(true)
			mgr := &fakeManager{state: state}
			d := &T{daemonsys: mgr, api: api, procs: procs}
			require.NoError(t, d.Stop(context.Background()))
			assert.Equal(t, []string{"ActiveState", "Stop"}, mgr.Calls())
			assert.Equal(t, 1, api.Posts(), "the daemon outside of the unit is stopped through its api")
			assert.Empty(t, procs.Signals())
		})
		t.Run(state+" with no daemon", func(t *testing.T) {
			api := &fakeAPI{}
			procs := &fakeProcesses{alive: map[int]bool{}}
			mgr := &fakeManager{state: state}
			d := &T{daemonsys: mgr, api: api, procs: procs}
			require.NoError(t, d.Stop(context.Background()))
			assert.Equal(t, 0, api.Posts())
		})
	}
}

// A job of the unit which did not end "done", as a stop another job
// replaced, is an error: the command must not say the daemon stopped.
func TestStopWithManagerJobNotDone(t *testing.T) {
	setupLifecycle(t)
	api, procs := newRunningDaemon(true)
	mgr := &fakeManager{state: "active", stopResult: "canceled"}
	d := &T{daemonsys: mgr, api: api, procs: procs}
	err := d.Stop(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "job canceled")
}

// The stop of the unit ExecStop waits for the daemon to exit, longer than a
// stop outside of systemd would, and never signals it: systemd escalates.
func TestStopFromManagerNeverKills(t *testing.T) {
	setupLifecycle(t)
	t.Run("slow stop", func(t *testing.T) {
		api, procs := newRunningDaemon(false)
		exitAfter := 4 * WaitStoppedTimeout
		go func() {
			time.Sleep(exitAfter)
			procs.exit(daemonPid)
		}()
		d := &T{daemonsys: &fakeManager{state: "deactivating", calledFromManager: true}, api: api, procs: procs}
		begin := time.Now()
		require.NoError(t, d.Stop(context.Background()))
		assert.GreaterOrEqual(t, time.Since(begin), exitAfter, "the stop waits for the daemon exit")
		assert.Empty(t, procs.Signals(), "no signal from ExecStop")
	})
	t.Run("api not answering", func(t *testing.T) {
		_, procs := newRunningDaemon(false)
		api := &fakeAPI{answers: false}
		d := &T{daemonsys: &fakeManager{state: "deactivating", calledFromManager: true}, api: api, procs: procs}
		require.NoError(t, d.Stop(context.Background()), "systemd sends the SIGTERM")
		assert.Equal(t, 1, api.Posts())
		assert.Empty(t, procs.Signals(), "no signal from ExecStop")
	})
}

// Outside of systemd, a stop request which fails when no daemon process is
// known is an error, and nothing is signaled.
func TestStopWithoutManagerPostErrorNoPidFile(t *testing.T) {
	setupLifecycle(t)
	api := &fakeAPI{answers: true}
	procs := &fakeProcesses{alive: map[int]bool{}}
	d := &T{api: api, procs: procs}
	err := d.Stop(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "post daemon stop")
	assert.Empty(t, procs.Signals())
}

// Outside of systemd, a daemon neither answering nor running is stopped.
func TestStopWithoutManagerNotRunning(t *testing.T) {
	setupLifecycle(t)
	api := &fakeAPI{}
	procs := &fakeProcesses{pidFile: daemonPid, alive: map[int]bool{}}
	d := &T{api: api, procs: procs}
	require.NoError(t, d.Stop(context.Background()))
	assert.Equal(t, 0, api.Posts())
	assert.Empty(t, procs.Signals())
}

// Outside of systemd, a daemon whose api does not answer is sent a SIGTERM,
// which it handles as a stop request.
func TestStopWithoutManagerPostErrorTerm(t *testing.T) {
	setupLifecycle(t)
	_, procs := newRunningDaemon(false)
	procs.onSignal = func(pid int, sig syscall.Signal) {
		if sig == syscall.SIGTERM {
			go procs.exit(pid)
		}
	}
	api := &fakeAPI{answers: false}
	d := &T{api: api, procs: procs}
	require.NoError(t, d.Stop(context.Background()))
	assert.Equal(t, []string{"terminated 4242"}, procs.Signals())
}

// Outside of systemd, a daemon still running after the wait is killed, as a
// last resort, and the stop says so with an error.
func TestStopWithoutManagerKillIsAnError(t *testing.T) {
	setupLifecycle(t)
	api, procs := newRunningDaemon(false)
	procs.onSignal = func(pid int, sig syscall.Signal) {
		if sig == syscall.SIGKILL {
			procs.mu.Lock()
			procs.alive[pid] = false
			procs.mu.Unlock()
		}
	}
	d := &T{api: api, procs: procs}
	err := d.Stop(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "killed")
	assert.Equal(t, []string{"killed 4242"}, procs.Signals())
}

// The pid the pid file names, given to another process after the daemon
// died, is not the daemon: it is neither taken for a running daemon nor
// signaled.
func TestStopWithoutManagerNeverSignalsAnotherProcess(t *testing.T) {
	setupLifecycle(t)
	procs := &fakeProcesses{
		pidFile:   daemonPid,
		alive:     map[int]bool{daemonPid: true},
		notDaemon: map[int]bool{daemonPid: true},
	}
	api := &fakeAPI{}
	d := &T{api: api, procs: procs}
	require.NoError(t, d.Stop(context.Background()))
	assert.Equal(t, 0, api.Posts())
	assert.Empty(t, procs.Signals())
}

// A restart from the command line under systemd returns when the restart
// job ended done, and the daemon runs.
func TestRestartFromCLIWaitsForTheJob(t *testing.T) {
	setupLifecycle(t)
	api, procs := newRunningDaemon(false)
	job := make(chan string)
	mgr := &fakeManager{state: "active", restartJob: job}
	d := &T{daemonsys: mgr, api: api, procs: procs}
	errC := make(chan error, 1)
	go func() { errC <- d.Restart(context.Background(), "") }()
	select {
	case err := <-errC:
		t.Fatalf("restart returned before the end of its job: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	job <- "done"
	require.NoError(t, <-errC)
	assert.Equal(t, []string{"ActiveState", "Restart"}, mgr.Calls())
}

// A restart job a stop replaced is canceled: the restart fails.
func TestRestartFromCLICanceledByAStop(t *testing.T) {
	setupLifecycle(t)
	api, procs := newRunningDaemon(false)
	mgr := &fakeManager{state: "active", restartResult: "canceled"}
	d := &T{daemonsys: mgr, api: api, procs: procs}
	err := d.Restart(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "job canceled")
}

// A restart the api asked for is queued from outside of the unit control
// group, and returns.
func TestRestartFromAPIUsesTheDetour(t *testing.T) {
	setupLifecycle(t)
	api, procs := newRunningDaemon(false)
	mgr := &fakeManager{state: "active"}
	d := &T{daemonsys: mgr, api: api, procs: procs}
	d.SetAPIDaemonPid(daemonPid)
	require.NoError(t, d.Restart(context.Background(), ""))
	assert.Equal(t, []string{"ActiveState", "RestartDetached"}, mgr.Calls())
}

// A restart of a unit not running the daemon stops the daemon started
// outside of it, then starts the unit, as v2 did.
func TestRestartWithManagerDaemonOutsideTheUnit(t *testing.T) {
	setupLifecycle(t)
	api, procs := newRunningDaemon(true)
	mgr := &fakeManager{state: "inactive", onStart: func() { api.setAnswers(true) }}
	d := &T{daemonsys: mgr, api: api, procs: procs}
	require.NoError(t, d.Restart(context.Background(), ""))
	assert.Equal(t, 1, api.Posts(), "the daemon outside of the unit is stopped")
	assert.Equal(t, []string{"ActiveState", "Start"}, mgr.Calls())
}

// A restart the api asked for, served after a stop which stopped the daemon
// asking, does nothing: the stop wins.
func TestRestartFromAPIAfterAStop(t *testing.T) {
	setupLifecycle(t)
	api := &fakeAPI{}
	procs := &fakeProcesses{alive: map[int]bool{}}
	t.Run("without manager", func(t *testing.T) {
		d := &T{api: api, procs: procs}
		d.SetAPIDaemonPid(daemonPid)
		require.NoError(t, d.Restart(context.Background(), ""))
		assert.Equal(t, 0, api.Posts())
	})
	t.Run("unit stopped", func(t *testing.T) {
		mgr := &fakeManager{state: "inactive"}
		d := &T{daemonsys: mgr, api: api, procs: procs}
		d.SetAPIDaemonPid(daemonPid)
		require.NoError(t, d.Restart(context.Background(), ""))
		assert.Equal(t, []string{"ActiveState"}, mgr.Calls(), "no start of the unit")
	})
}
