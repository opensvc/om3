package daemoncmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/gommon/log"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/hbsecobject"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemon"
	"github.com/opensvc/om3/v3/daemon/daemonsys"
	"github.com/opensvc/om3/v3/drivers/resfsflag"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/lock"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/waitfor"
)

var (
	// lockPath is the lock a starting daemon holds until it is started,
	// and a stop holds while it stops the daemon: a stop waits for a
	// starting daemon to be started, so a daemon whose api does not answer
	// to a stop holding it is not starting, but hung or stopping. It is
	// daemonLockPath when empty.
	//
	// It is in the lock directory of the agent, which root alone can
	// write in. It was /tmp/locks/main: a user making /tmp/locks first, as
	// after a reboot cleaned /tmp, could have the lock of each start and
	// stop write and truncate any file through a link, or hold the lock.
	lockPath string

	// cliLockPath is the lock of the lifecycle commands run outside of
	// systemd: a start holds it from the spawn of the daemon to its
	// readiness, a restart from the stop of the daemon to the readiness of
	// the new one, and a stop while it stops. A stop requested during a
	// restart waits for it to end, then stops: the stop wins. It is
	// daemonLockPath with a "-cli" suffix when empty.
	cliLockPath string

	lockTimeout        = 60 * time.Second
	WaitRunningTimeout = 20 * time.Second
	WaitRunningDelay   = 500 * time.Millisecond

	// WaitStoppedTimeout is how long a stop run outside of systemd waits
	// for the daemon to exit before it kills it. A daemon stops in a few
	// seconds, but in more than the 4 seconds this wait was on a loaded
	// node, which had it killed during a restart. A daemon not stopped in
	// a minute, the time the lifecycle commands wait for their lock, is
	// hung. Under systemd, the stop of the unit ExecStop has no limit of
	// its own: the TimeoutStopSec of the unit bounds it.
	WaitStoppedTimeout = time.Minute
	WaitStoppedDelay   = 250 * time.Millisecond

	// WaitKilledTimeout is how long a stop waits for a daemon it killed
	// to be gone.
	WaitKilledTimeout = 5 * time.Second

	errGoTest = errors.New("running from go test")

	ErrAlreadyRunning = errors.New("daemon already running")
)

type (
	T struct {
		client    *client.T
		node      string
		daemonsys Manager
		api       daemonAPI
		procs     processes

		// apiDaemonPid is the pid of the daemon which asked for its
		// restart through its api, and forked this command. Set, the
		// restart is queued to systemd from outside of the unit, and
		// returns, and it is done only if that daemon still runs: a stop
		// requested meanwhile wins.
		apiDaemonPid int
	}

	// Manager is the service manager running the daemon, systemd.
	Manager interface {
		// ActiveState returns the ActiveState of the unit: "active",
		// "activating", "deactivating", "reloading", "inactive",
		// "failed".
		ActiveState(ctx context.Context) (string, error)
		CalledFromManager() bool
		Close() error
		Defined(ctx context.Context) (bool, error)

		// Start, Restart and Stop return when the job of the unit
		// ended, and an error when its result is not "done".
		Start(ctx context.Context) error
		Restart(ctx context.Context) error
		Stop(context.Context) error

		// RestartDetached queues a restart of the unit, if it runs, from
		// outside of the unit, and returns.
		RestartDetached() error

		IsSystemStopping() (bool, error)
	}

	// daemonAPI is the api of the local daemon, as the lifecycle commands
	// use it.
	daemonAPI interface {
		// Ping says the api answers.
		Ping(ctx context.Context) bool

		// PostStop asks the daemon to stop, and returns its pid.
		PostStop(ctx context.Context) (int, error)
	}

	// processes tells about the daemon process, and signals it.
	processes interface {
		// DaemonPid returns the pid the pid file names, when it is the one
		// of an "om daemon run" process, else -1.
		DaemonPid() (int, error)

		// PidFileNames says the pid file names pid. The daemon removes it
		// as it exits.
		PidFileNames(pid int) bool

		// Alive says the process pid exists, and is not a zombie.
		Alive(pid int) (bool, error)

		Signal(pid int, sig syscall.Signal) error
	}

	apiClient struct {
		c *client.T
	}

	osProcesses struct{}
)

func bootStrapCcfg() error {
	log := logger("bootstrap cluster config: ")
	type mandatoryKeyT struct {
		Key       key.T
		Default   string
		Obfuscate bool
	}
	keys := []mandatoryKeyT{
		{
			Key:       key.New("cluster", "id"),
			Default:   uuid.New().String(),
			Obfuscate: false,
		},
		{
			Key:       key.New("cluster", "name"),
			Default:   naming.Random(),
			Obfuscate: false,
		},
		{
			Key:       key.New("cluster", "nodes"),
			Default:   hostname.Hostname(),
			Obfuscate: false,
		},
		{
			Key:       key.New("cluster", "secret"),
			Default:   strings.ReplaceAll(uuid.New().String(), "-", ""),
			Obfuscate: true,
		},
	}

	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}

	for _, k := range keys {
		if ccfg.Config().Get(k.Key) != "" {
			continue
		}
		op := keyop.New(k.Key, keyop.Set, k.Default, 0)
		if err := ccfg.Config().PrepareSet(*op); err != nil {
			return err
		}
		if k.Obfuscate {
			op.Value = "xxxx"
		}
		log.Infof("%s", op)
	}

	// Prepares futures node join, because it requires at least one heartbeat.
	// So on cluster config where no hb exists, we automatically set hb#1.type=unicast.
	hasHbSection := false
	for _, section := range ccfg.Config().SectionStrings() {
		if strings.HasPrefix(section, "hb") {
			hasHbSection = true
			break
		}
	}
	if !hasHbSection {
		k := key.New("hb#1", "type")
		op := keyop.New(k, keyop.Set, "unicast", 0)
		if err := ccfg.Config().PrepareSet(*op); err != nil {
			return err
		}
		log.Infof("add default heartbeat: %s", op)
	}

	if err := ccfg.Config().Commit(); err != nil {
		return err
	}
	if cfg, err := object.SetClusterConfig(); err != nil {
		return err
	} else {
		for _, issue := range cfg.Issues {
			log.Warnf("issue: %s", issue)
		}
	}

	if secret := ccfg.Config().Get(key.New("cluster", "secret")); secret != "" {
		if err := bootStrapSecHb(secret, 0); err != nil {
			return err
		}
	}
	return nil
}

func bootStrapSecHb(currentSecret string, currentVersion uint64) error {
	log := logger("bootstrap heartbeat secret")
	type keyT struct {
		Name      string
		Value     string
		Obfuscate bool
	}
	keys := []keyT{
		{
			Name:      hbsecobject.Secret,
			Value:     currentSecret,
			Obfuscate: true,
		},
		{
			Name:      hbsecobject.Version,
			Value:     fmt.Sprintf("%d", currentVersion),
			Obfuscate: false,
		},
		{
			Name:      hbsecobject.AltSecret,
			Value:     currentSecret,
			Obfuscate: true,
		},
		{
			Name:      hbsecobject.AltVersion,
			Value:     fmt.Sprintf("%d", currentVersion),
			Obfuscate: false,
		},
	}

	secHb, err := object.NewSec(naming.SecHb, object.WithVolatile(false))
	if err != nil {
		return err
	}

	existingKeys, err := secHb.AllKeys()
	if err != nil {
		return err
	}
	var changed bool
	for _, k := range keys {
		if slices.Contains(existingKeys, k.Name) {
			// already exists, skipped
			continue
		}
		log.Infof("%s adding key %s", secHb, k.Name)
		if err := secHb.TransactionAddKey(k.Name, []byte(k.Value)); err != nil {
			return fmt.Errorf("can't add key %s: %w", k.Name, err)
		}
		changed = true
	}
	if changed {
		log.Infof("%s commit changes ...", secHb)
		if err := secHb.Config().Commit(); err != nil {
			return fmt.Errorf("can't commit %s changes: %w", secHb, err)
		}
	}
	return nil
}

func New(c *client.T) *T {
	return &T{
		client: c,
		api:    apiClient{c: c},
		procs:  osProcesses{},
	}
}

// SetAPIDaemonPid says the command was forked by the daemon of pid, to
// serve a restart its api received.
func (t *T) SetAPIDaemonPid(pid int) {
	t.apiDaemonPid = pid
}

func (t *T) LoadManager(ctx context.Context) error {
	var i any
	i, err := daemonsys.New(ctx)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// skip: Error: dial unix /run/systemd/private: connect: no such file or directory
			return nil
		}
		return err
	}
	if mgr, ok := i.(Manager); ok {
		t.daemonsys = mgr
	}
	return nil
}

// Restart restarts the daemon, and returns when it runs again.
//
// Under systemd, the restart is a job of the unit, which a stop requested
// meanwhile replaces: the restart then fails, and the stop wins. A restart
// the api asked for, see SetAPIDaemonPid, is queued from outside of the
// unit, and returns at once.
//
// Without systemd, the restart stops then starts the daemon, holding the
// cli lock: a stop requested meanwhile waits for the restart to end, then
// stops.
func (t *T) Restart(ctx context.Context, profile string) error {
	if t.daemonsys == nil {
		return t.restartWithoutManager(profile)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		return t.restartWithoutManager(profile)
	}
	// note: always ask manager for restart (during POST /daemon/restart handler
	// the server api is probably CalledFromManager). And systemd unit doesn't define
	// restart command.
	return t.restartWithManager(ctx)
}

func (t *T) SetNode(node string) {
	t.node = node
}

// run function will start daemon with internal lock protection
func (t *T) run(ctx context.Context) error {
	log := logger("locked run: ")

	// Catch the stop signals from the beginning of the startup: the daemon
	// watches them once its bus runs, and until then, a SIGTERM, as the
	// one systemd sends to stop the unit while it is activating, must end
	// the startup cleanly rather than kill the process. Both handlers
	// receive the signals until the daemon is started, and the daemon
	// takes the first stop request only. Once started, the daemon alone
	// handles them, as before: a signal after its stop request has the
	// default action.
	sigCtx, sigStop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer sigStop()
	stoppedDuringStart := func() bool {
		if sigCtx.Err() == nil {
			return false
		}
		log.Infof("stop requested during the startup: stopped")
		return true
	}

	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		return fmt.Errorf("create mandatory directories: %w", err)
	}
	if err := resfsflag.MakeBaseDir(log.Warnf); err != nil {
		log.Warnf("fs.flag base directory: %s", err)
	}
	setupCrashReport()
	unlock, err := getLock("Run")
	if err != nil {
		return err
	}
	release := sync.OnceFunc(unlock)
	defer release()
	isRunning, err := t.isRunning()
	if err != nil {
		return err
	}
	if isRunning {
		return ErrAlreadyRunning
	}
	pidFile := daemonPidFile()
	log.Tracef("create pid file %s", pidFile)

	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return err
	}
	defer func() {
		log.Tracef("remove pid file %s", pidFile)
		if err := os.Remove(pidFile); err != nil {
			log.Errorf("remove pid file %s: %s", pidFile, err)
		}
	}()
	// The scan is not interrupted: it saves what it found, which must not
	// be the partial result of an interrupted scan.
	if err := capabilities.Scan(ctx); err != nil {
		return err
	}
	log.Attr("capabilities", capabilities.Data()).Infof("rescanned node capabilities")
	if stoppedDuringStart() {
		return nil
	}

	if err := bootStrapCcfg(); err != nil {
		log.Tracef("bootstrap cluster config %s", err)
		return err
	}
	if stoppedDuringStart() {
		return nil
	}
	d := daemon.New()
	log.Tracef("starting daemon...")
	err = d.Start(context.Background())
	release()
	signalCaught := sigCtx.Err() != nil
	sigStop()
	if err == daemon.ErrStoppedDuringStart {
		log.Infof("stopped during the startup")
		return nil
	} else if err != nil {
		return err
	}
	if signalCaught {
		// A signal caught before the daemon watched them, or caught
		// by both, which the daemon takes once.
		d.RequestStop("signal")
	}
	log.Infof("started")
	d.Wait()
	log.Infof("stopped")
	return nil
}

// Start handle daemon start from command origin.
//
// It is used to forward start control to (systemd) manager (when the origin is not systemd)
func (t *T) Start(ctx context.Context, profile string) error {
	if t.daemonsys == nil {
		return t.startWithoutManager(profile)
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		return t.startWithoutManager(profile)
	} else if t.daemonsys.CalledFromManager() {
		return t.startFromManager(profile)
	} else {
		return t.startWithManager(ctx)
	}
}

func (t *T) startFromManager(profile string) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return nil
	}
	log.Infof("exec run (origin manager)")
	return t.start(profile)
}

// Stop stops the daemon.
//
// Under systemd, the stop is a job of the unit, which replaces a start or a
// restart in progress: the stop wins. Called by the unit ExecStop, it asks
// the daemon to stop, and waits for it to exit, systemd escalating when it
// does not. Without systemd, it holds the cli lock, so a stop requested
// during a restart waits for it to end, then stops.
func (t *T) Stop(ctx context.Context) error {
	if t.daemonsys == nil {
		return t.StopWithoutManager()
	}
	defer func() {
		_ = t.daemonsys.Close()
	}()
	if ok, err := t.daemonsys.Defined(ctx); err != nil || !ok {
		return t.StopWithoutManager()
	}
	if t.daemonsys.CalledFromManager() {
		return t.stopFromManager()
	}
	return t.stopWithManager(ctx)
}

// IsRunning function detect daemon status using api
//
// it returns true is daemon is running, else false
func (t *T) IsRunning() (bool, error) {
	return t.isRunning()
}

// WaitRunning function waits for daemon running
//
// It needs to be called from a cli lock protection
func (t *T) WaitRunning() error {
	if ok, err := waitfor.TrueNoError(WaitRunningTimeout, WaitRunningDelay, t.IsRunning); err != nil {
		return fmt.Errorf("wait running: %s", err)
	} else if !ok {
		return fmt.Errorf("wait running: timeout")
	}
	return nil
}

// getLock() manage internal lock for functions that will stop/start/restart daemon
//
// It returns a release function to release lock
func getLock(desc string) (func(), error) {
	return lock.Lock(daemonLockPath(), lockTimeout, desc)
}

// daemonLockPath returns the path of the lock of the daemon lifecycle.
func daemonLockPath() string {
	if lockPath != "" {
		return lockPath
	}
	return filepath.Join(rawconfig.Paths.Lock, "daemon")
}

// daemonCLILockPath returns the path of the lock of the lifecycle commands
// run outside of systemd.
func daemonCLILockPath() string {
	if cliLockPath != "" {
		return cliLockPath
	}
	return daemonLockPath() + "-cli"
}

// stopLockTimeout is how long a stop waits for the cli lock, which a restart
// holds for its stop and the wait of the new daemon running.
func stopLockTimeout() time.Duration {
	return lockTimeout + WaitStoppedTimeout + WaitKilledTimeout + WaitRunningTimeout
}

// StopWithoutManager stops the daemon run outside of systemd, holding the
// cli lock: a stop requested during a restart waits for its end, then stops.
func (t *T) StopWithoutManager() error {
	return lock.Func(daemonCLILockPath(), stopLockTimeout(), "daemon stop", func() error {
		return t.stopMainLocked(false)
	})
}

// stopMainLocked stops the daemon holding the main lock, which a starting
// daemon holds until it is started: no start is in progress during the stop.
func (t *T) stopMainLocked(fromManager bool) error {
	release, err := getLock("Stop")
	if err != nil {
		return err
	}
	defer release()
	return t.stop(fromManager)
}

// start starts a daemon, holding the cli lock until it runs.
func (t *T) start(profile string) error {
	return lock.Func(daemonCLILockPath(), lockTimeout, "daemon start", func() error {
		if isRunning, err := t.isRunning(); err != nil {
			return err
		} else if isRunning {
			return ErrAlreadyRunning
		}
		return t.spawn(profile)
	})
}

// spawn runs "om daemon run" in the background, and waits for the daemon to
// run. The caller holds the cli lock.
func (t *T) spawn(profile string) error {
	args := []string{"daemon", "run"}
	if profile != "" {
		args = append(args, "--cpuprofile", profile)
	}
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(args),
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := t.WaitRunning(); err != nil {
		return fmt.Errorf("start checker wait running failed: %w", err)
	}
	return nil
}

func (t *T) startWithoutManager(profile string) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return nil
	}
	return t.start(profile)
}

func (t *T) startWithManager(ctx context.Context) error {
	if isRunning, err := t.isRunning(); err != nil {
		return err
	} else if isRunning {
		return ErrAlreadyRunning
	}
	return t.startUnit(ctx)
}

// startUnit starts the unit, and waits for the daemon to run.
func (t *T) startUnit(ctx context.Context) error {
	if err := t.daemonsys.Start(ctx); err != nil {
		return fmt.Errorf("daemonsys start: %w", err)
	}
	if err := t.WaitRunning(); err != nil {
		return fmt.Errorf("wait running: %w", err)
	}
	return nil
}

func (t *T) restartWithoutManager(profile string) error {
	return t.restartLocked(func() error {
		if isRunning, err := t.isRunning(); err != nil {
			return err
		} else if isRunning {
			return nil
		}
		return t.spawn(profile)
	})
}

// restartLocked stops the daemon, then calls start, holding the cli lock:
// a stop requested meanwhile, which takes it too, waits for the restart to
// end, then stops. A restart the api asked for, which a stop took the lock
// before, finds the daemon which asked for it stopped, and does nothing: the
// stop wins either way.
func (t *T) restartLocked(start func() error) error {
	log := logger("restart: ")
	return lock.Func(daemonCLILockPath(), lockTimeout, "daemon restart", func() error {
		if t.apiDaemonPid > 0 {
			if stopped, err := t.stopped(t.apiDaemonPid); err != nil {
				return err
			} else if stopped {
				log.Infof("daemon pid %d, which asked for the restart, stopped meanwhile: no restart", t.apiDaemonPid)
				return nil
			}
		}
		if err := t.stopMainLocked(false); err != nil {
			return err
		}
		return start()
	})
}

// unitRunsDaemon says a unit in the ActiveState state runs a daemon, or is
// starting or stopping one.
func unitRunsDaemon(state string) bool {
	switch state {
	case "inactive", "failed", "":
		return false
	default:
		return true
	}
}

// restartWithManager restarts the unit.
//
// From the command line, it waits for the end of the restart job, which must
// be done, as a restart a stop replaced is canceled, and for the daemon to
// run. Forked by the daemon to serve its api, it lives in the unit control
// group, and queues the restart through systemd-run.
//
// When the unit is not running, a daemon answering was started outside of it,
// and a start of the unit would fail on it: as v2 did, it is stopped, then the
// unit is started.
func (t *T) restartWithManager(ctx context.Context) error {
	log := logger("restart: ")
	state, err := t.daemonsys.ActiveState(ctx)
	if err != nil {
		return fmt.Errorf("can't detect the unit active state: %w", err)
	}
	if !unitRunsDaemon(state) {
		return t.restartLocked(func() error {
			return t.startUnit(ctx)
		})
	}
	if t.apiDaemonPid > 0 {
		log.Infof("queue the restart of the unit")
		if err := t.daemonsys.RestartDetached(); err != nil {
			return fmt.Errorf("daemonsys restart detached: %w", err)
		}
		return nil
	}
	if err := t.daemonsys.Restart(ctx); err != nil {
		return fmt.Errorf("daemonsys restart: %w", err)
	}
	if err := t.WaitRunning(); err != nil {
		return fmt.Errorf("wait running: %w", err)
	}
	return nil
}

// stopWithManager stops the unit. The stop job replaces a start or a restart
// of the unit, running or queued: the stop wins.
//
// When the unit was not running, a daemon started outside of it, which
// systemd can't stop, is stopped without it.
func (t *T) stopWithManager(ctx context.Context) error {
	state, err := t.daemonsys.ActiveState(ctx)
	if err != nil {
		return fmt.Errorf("can't detect the unit active state: %w", err)
	}
	if err := t.daemonsys.Stop(ctx); err != nil {
		return fmt.Errorf("daemonsys stop: %w", err)
	}
	if unitRunsDaemon(state) {
		return nil
	}
	return t.StopWithoutManager()
}

func (t *T) stopFromManager() error {
	isSystemStopping, err := t.daemonsys.IsSystemStopping()
	if err != nil {
		return err
	}

	if isSystemStopping {
		fmt.Printf("the operating system is stopping: promote to daemon shutdown")
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithVarArgs("daemon", "shutdown"),
		)
		cmd.Cmd().Stdout = io.Discard
		cmd.Cmd().Stderr = io.Discard
		return cmd.Run()
	}
	return t.stopMainLocked(true)
}

// isAnswerLost says the error of a stop request is the connection the
// daemon closed as it stopped, after it received the request.
func isAnswerLost(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) ||
		strings.Contains(err.Error(), "unexpected EOF") ||
		strings.Contains(err.Error(), "unexpected end of JSON input")
}

// stop asks the daemon to stop through its api, and waits for it to exit.
// The caller holds the main lock.
//
// Called by the unit ExecStop (fromManager), the stop never signals the
// daemon: it waits for its exit with no limit of its own, as the
// TimeoutStopSec of the unit bounds the wait, and systemd escalates. When
// the api does not answer, it returns at once, and systemd sends the daemon
// the SIGTERM it handles as a stop request.
//
// Outside of systemd, when the api does not answer, the daemon process the
// pid file names is sent a SIGTERM. A daemon still running WaitStoppedTimeout
// after the request is killed, as a last resort, and the stop returns an
// error, as it does when it can't confirm the stop.
func (t *T) stop(fromManager bool) error {
	log := logger("stop: ")
	ctx := context.Background()
	pid, err := t.daemonPid()
	if err != nil {
		return err
	}
	if pid < 0 && !t.api.Ping(ctx) {
		log.Debugf("not running")
		return nil
	}
	daemonPid, err := t.api.PostStop(ctx)
	switch {
	case err == nil:
		pid = daemonPid
	case isAnswerLost(err):
		log.Debugf("post daemon stop: %s: the daemon is stopping", err)
		if pid < 0 {
			return t.waitAPIDown(fromManager)
		}
	case fromManager:
		log.Warnf("post daemon stop: %s: let systemd terminate the daemon", err)
		return nil
	case pid < 0:
		return fmt.Errorf("post daemon stop: %w", err)
	default:
		log.Warnf("post daemon stop: %s: send a SIGTERM to the daemon pid %d", err, pid)
		if err := t.procs.Signal(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("signal the daemon pid %d: %w", pid, err)
		}
	}
	return t.waitStopped(pid, fromManager)
}

// daemonPid returns the pid of the daemon process, -1 for none.
func (t *T) daemonPid() (int, error) {
	pid, err := t.procs.DaemonPid()
	if err != nil {
		return -1, err
	}
	if pid <= 0 || pid == os.Getpid() {
		// os.Getpid(): avoids false positives caused by remnants of a
		// previous daemon PID after a fast OS reboot.
		return -1, nil
	}
	if alive, err := t.procs.Alive(pid); err != nil {
		return -1, err
	} else if !alive {
		return -1, nil
	}
	return pid, nil
}

// stopped says the daemon of pid stopped: its process is gone, or the pid
// file it removes as it exits no longer names it.
func (t *T) stopped(pid int) (bool, error) {
	if alive, err := t.procs.Alive(pid); err != nil {
		return false, err
	} else if !alive {
		return true, nil
	}
	return !t.procs.PidFileNames(pid), nil
}

// waitStopped waits for the daemon of pid to stop: with no limit when called
// by the manager, else for WaitStoppedTimeout, then it kills it.
func (t *T) waitStopped(pid int, fromManager bool) error {
	isStopped := func() (bool, error) {
		return t.stopped(pid)
	}
	if fromManager {
		for {
			if ok, err := isStopped(); err != nil {
				return err
			} else if ok {
				return nil
			}
			time.Sleep(WaitStoppedDelay)
		}
	}
	if ok, err := waitfor.TrueNoError(WaitStoppedTimeout, WaitStoppedDelay, isStopped); err != nil {
		return fmt.Errorf("wait for the daemon pid %d to stop: %w", pid, err)
	} else if ok {
		return nil
	}
	return t.killLastResort(pid)
}

// waitAPIDown waits for the api of a daemon stopping, whose pid is unknown,
// to no longer answer.
func (t *T) waitAPIDown(fromManager bool) error {
	isDown := func() (bool, error) {
		return !t.api.Ping(context.Background()), nil
	}
	if fromManager {
		for t.api.Ping(context.Background()) {
			time.Sleep(WaitStoppedDelay)
		}
		return nil
	}
	if ok, _ := waitfor.TrueNoError(WaitStoppedTimeout, WaitStoppedDelay, isDown); !ok {
		return fmt.Errorf("the daemon api still answers %s after the stop request: stop not confirmed", WaitStoppedTimeout)
	}
	return nil
}

// killLastResort kills the daemon of pid, still running after a stop
// request, and returns an error saying so. It kills only the "om daemon run"
// process the pid file names, never a pid the system gave another process.
func (t *T) killLastResort(pid int) error {
	log := logger("stop: ")
	if current, err := t.daemonPid(); err != nil {
		return fmt.Errorf("daemon pid %d still running %s after the stop request: %w", pid, WaitStoppedTimeout, err)
	} else if current != pid {
		return fmt.Errorf("daemon pid %d still running %s after the stop request, and not the daemon the pid file names: not killed, stop not confirmed", pid, WaitStoppedTimeout)
	}
	log.Warnf("daemon pid %d still running %s after the stop request: kill", pid, WaitStoppedTimeout)
	if err := t.procs.Signal(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("kill the daemon pid %d: %w", pid, err)
	}
	isGone := func() (bool, error) {
		alive, err := t.procs.Alive(pid)
		return !alive, err
	}
	if ok, err := waitfor.TrueNoError(WaitKilledTimeout, WaitStoppedDelay, isGone); err != nil {
		return fmt.Errorf("daemon pid %d killed: %w", pid, err)
	} else if !ok {
		return fmt.Errorf("daemon pid %d still running %s after its kill", pid, WaitKilledTimeout)
	}
	return fmt.Errorf("daemon pid %d did not stop within %s after the stop request: killed", pid, WaitStoppedTimeout)
}

func (t *T) startProfile(profile string) (func(), error) {
	f, err := os.Create(profile)
	if err != nil {
		return nil, fmt.Errorf("create CPU profile: %w", err)
	}
	stop := func() {
		_ = f.Close()
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		return stop, fmt.Errorf("start CPU profile: %w", err)
	}
	stop = func() {
		pprof.StopCPUProfile()
		_ = f.Close()
	}
	return stop, nil
}

func (t *T) Run(ctx context.Context, profile string) error {
	if profile != "" {
		if stopProfile, err := t.startProfile(profile); err != nil {
			return err
		} else {
			defer stopProfile()
		}
	}
	if err := t.run(ctx); err != nil {
		return err
	}
	return nil
}

// Ping says the api of the local daemon answers.
func (a apiClient) Ping(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	resp, err := a.c.GetNodePing(ctx, hostname.Hostname())
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusNoContent
}

// PostStop asks the local daemon to stop, and returns its pid.
func (a apiClient) PostStop(ctx context.Context) (int, error) {
	resp, err := a.c.PostDaemonStopWithResponse(ctx, hostname.Hostname())
	if err != nil {
		return -1, err
	}
	if resp.JSON200 == nil {
		return -1, fmt.Errorf("unexpected status code: %s body: %s", resp.Status(), string(resp.Body))
	}
	return resp.JSON200.Pid, nil
}

func (osProcesses) DaemonPid() (int, error) {
	pid, err := getPid()
	if errors.Is(err, errGoTest) {
		// The daemon of the go tests runs in the test process: none to
		// signal.
		return -1, nil
	}
	return pid, err
}

func (osProcesses) PidFileNames(pid int) bool {
	v, err := extractPidFromPidFile(daemonPidFile())
	return err == nil && v == pid
}

func (osProcesses) Alive(pid int) (bool, error) {
	return isProcessAlive(pid)
}

func (osProcesses) Signal(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

// isProcessAlive says the process pid exists and is not a zombie, which a
// dead daemon is until its parent reaps it.
func isProcessAlive(pid int) (bool, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// pid (comm) state ...: comm may hold spaces and parentheses.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return false, fmt.Errorf("/proc/%d/stat: no comm end", pid)
	}
	fields := strings.Fields(string(b[i+1:]))
	if len(fields) == 0 {
		return false, fmt.Errorf("/proc/%d/stat: no state", pid)
	}
	switch fields[0] {
	case "Z", "X":
		return false, nil
	}
	return true, nil
}

// isRunning returns true if the daemon api responds or if process id from
// daemon pid file exists, matching 'daemon run' and is not self pid.
func (t *T) isRunning() (bool, error) {
	if t.api.Ping(context.Background()) {
		return true, nil
	}
	pid, err := t.daemonPid()
	if err != nil {
		return false, err
	}
	return pid > 0, nil
}

func getPid() (int, error) {
	pidFile := daemonPidFile()
	pid, err := extractPidFromPidFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return -1, nil
	} else if errors.Is(err, syscall.ESRCH) {
		return -1, nil
	} else if err != nil {
		return -1, err
	}
	if leader, err := isThreadGroupLeader(pid); err != nil {
		return -1, err
	} else if !leader {
		// A thread, not a process: at boot, the pid of the daemon of a
		// previous boot can be the one of a thread of the daemon starting,
		// with its command line, which would read "already running".
		return -1, nil
	}
	v, err := isCmdlineMatchingDaemon(pid)
	if !v {
		return -1, err
	}
	return pid, err
}

// isThreadGroupLeader says the id pid is the one of a process, and not of
// one of the threads of a process, which /proc also shows: false for an id
// unknown.
func isThreadGroupLeader(pid int) (bool, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "Tgid:"); ok {
			tgid, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return false, fmt.Errorf("/proc/%d/status: %w", pid, err)
			}
			return tgid == pid, nil
		}
	}
	return false, fmt.Errorf("/proc/%d/status: no Tgid", pid)
}

func isCmdlineMatchingDaemon(pid int) (bool, error) {
	log := logger("validate proc: ")
	getPidInfo := func(pid int) (args []string, running bool, err error) {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))

		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		} else if errors.Is(err, syscall.ESRCH) {
			return nil, false, nil
		} else if err != nil {
			return nil, false, err
		} else if strings.Contains(string(b), "/daemoncmd.test") {
			return nil, true, errGoTest
		}
		if len(b) == 0 {
			return nil, false, nil
		}
		sep := make([]byte, 1)
		l := bytes.Split(b, sep)
		args = make([]string, len(l))
		for i, b := range l {
			args[i] = string(b)
		}
		return args, true, nil
	}

	areProcArgsMatching := func(args []string) bool {
		if len(args) == 0 {
			log.Tracef("process %d pointed by %s ran by a command with no arguments", pid, daemonPidFile())
			return false
		}
		if len(args) < 3 {
			log.Tracef("process %d pointed by %s ran by a command with too few arguments: %s", pid, daemonPidFile(), args)
			return false
		}
		if args[1] != "daemon" || args[2] != "run" {
			log.Tracef("process %d pointed by %s is not a om daemon: %s", pid, daemonPidFile(), args)
			return false
		}
		return true
	}

	if l, running, err := getPidInfo(pid); err != nil {
		return false, err
	} else if !running {
		return false, nil
	} else if len(l) == 0 {
		// need rescan, pid is detected, but read the read /proc/%d/cmdline may returns empty []byte
		//     om[364661]: daemon: main: daemon started
		//     om[364661]: daemon: cmd: locked start: started
		//     ...
		//     om[368219]: daemon: cmd: cli restart: origin os, no unit defined
		//     om[364661]: daemon: main: stopping on daemon ctl message
		//     om[364661]: daemon: main: daemon stopping
		//     om[368219]: daemon: cmd: start from cmd: wait running: start checker wait
		//                 running failed: wait running: process 364661 pointed
		//                 by /var/lib/opensvc/osvcd.pid ran by a command with too few arguments: []
		time.Sleep(500 * time.Millisecond)
		if l, running, err := getPidInfo(pid); err != nil {
			return false, err
		} else if !running {
			return false, nil
		} else {
			return areProcArgsMatching(l), nil
		}
	} else {
		return areProcArgsMatching(l), nil
	}
}

func extractPidFromPidFile(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1, err
	}
	return pid, nil
}

func daemonPidFile() string {
	return filepath.Join(rawconfig.Paths.Var, "osvcd.pid")
}

func logger(s string) *plog.Logger {
	return plog.NewDefaultLogger().
		Attr("pkg", "daemon/daemoncmd").
		WithPrefix(fmt.Sprintf("daemon: cmd: %s", s))
}
