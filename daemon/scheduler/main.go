package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/kwoption"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/resourcereqs"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/metricsreg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/runfiles"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type (
	T struct {
		ctx       context.Context
		cancel    context.CancelFunc
		log       *plog.Logger
		localhost string
		databus   *daemondata.T
		publisher pubsub.Publisher

		jobs    jobs
		enabled bool

		// timer wakes the loop when the soonest job is due, or after
		// maxSleep at most.
		timer *time.Timer

		// now and exec are the clock and the job runner, which the tests
		// replace.
		now  func() time.Time
		exec func(schedule.Entry) error

		provisioned map[naming.Path]bool
		failover    map[naming.Path]bool

		// notSource says why the local instance of an object is not
		// the one its data is replicated from, "" when it is, for the
		// jobs requiring the replication source. It is updated from
		// the local InstanceStatusUpdated events.
		notSource map[naming.Path]string

		// localStatus is the last status of the local instance of each
		// object, the requirements of its jobs are evaluated from.
		localStatus         map[naming.Path]instance.Status
		schedules           Schedules
		isCollectorJoinable bool

		wg sync.WaitGroup

		// running counts the jobs running in the background.
		running sync.WaitGroup

		subQS pubsub.QueueSizer

		status daemonsubsystem.Scheduler

		maxRunning int

		// lastRunOnAllPeers stores the schedule most recent run time
		// whatever the node. Used to avoid running again too soon
		// after a failover.
		//
		// This map is updated from the InstanceStatusUpdated events
		// received via ObjectStatusUpdated.SrcEv for peers and from
		// job execution for the local node.
		lastRunOnAllPeers timeMap

		// reqSatisfied stores which schedule entry has unsatisfied
		// requirements, blocking its scheduling.
		//
		// This map is updated from the InstanceStatusUpdated events
		// received via ObjectStatusUpdated.SrcEv for the local node.
		reqSatisfied errMap
	}

	Schedules map[naming.Path]map[string]schedule.Entry

	// pathKeyMap holds a value per object and schedule key.
	pathKeyMap[V any] map[naming.Path]map[string]V

	timeMap = pathKeyMap[time.Time]
	errMap  = pathKeyMap[error]
)

const (
	// maxSleep is the longest the loop sleeps without looking at the
	// clock. The timers count the time elapsed, which a suspend or a step
	// of the wall clock leaves behind: a job due at a wall time is taken
	// within maxSleep of it whatever happened to the clock meanwhile.
	maxSleep = time.Minute

	// lateTolerance is how late a job is taken and still counts as on
	// time. The next run of a job taken on time is planned from the time
	// it was due, so its runs do not drift. The next run of a job taken
	// later is planned from the time it is taken: the runs it missed, while
	// the node was suspended for example, are not made up in a burst.
	lateTolerance = 10 * time.Second

	// holdRetryDelay is how soon a job held back by an action of its
	// instance is tried again.
	holdRetryDelay = time.Second
)

var (
	incompatibleNodeMonitorStatus = map[node.MonitorState]any{
		node.MonitorStateInit:             nil,
		node.MonitorStateMaintenance:      nil,
		node.MonitorStateRejoin:           nil,
		node.MonitorStateShutdownProgress: nil,
		node.MonitorStateUpgrade:          nil,
	}

	// The two object scoped counters carry a path label, so they grow
	// with the cluster: 288 of the 292 scheduler series on a 103 object
	// cluster. They go to /metrics/scheduler.
	jobRunByPathKeyCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "scheduler",
			Name:      "object_job_runs_total",
			Help:      "The number of schedule entry runs, by object and schedule key",
		}, []string{"action", "path", "key"})

	jobRunByPathCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "scheduler",
			Name:      "object_runs_total",
			Help:      "The number of schedule entry runs, by object",
		}, []string{"action", "path"})

	// jobRunCount is bounded by the number of actions, so it stays on the
	// default registry, where it is the hint: a rise in failures or a
	// flat line where runs were expected is what sends you to
	// /metrics/scheduler to find which object.
	jobRunCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "scheduler",
			Name:      "runs_total",
			Help:      "The number of schedule entry runs, by action (per object at /metrics/scheduler)",
		}, []string{"action"})
)

func init() {
	metricsreg.Scheduler.MustRegister(jobRunByPathKeyCount, jobRunByPathCount)
}

func (t Schedules) Del(path naming.Path, key string) {
	if m, ok := t[path]; ok {
		delete(m, key)
	}
}

func (t Schedules) DelPath(path naming.Path) {
	delete(t, path)
}

func (t Schedules) Add(path naming.Path, e schedule.Entry) {
	if _, ok := t[path]; !ok {
		t[path] = make(map[string]schedule.Entry)
	}
	t[path][e.Key] = e
}

func (t Schedules) Table(path naming.Path) (l schedule.Table) {
	m, ok := t[path]
	if !ok {
		return
	}
	for _, entry := range m {
		l = append(l, entry)
	}
	return
}

func (t Schedules) Get(path naming.Path, k string) (schedule.Entry, bool) {
	if m, ok := t[path]; !ok {
		return schedule.Entry{}, false
	} else if e, ok := m[k]; !ok {
		return schedule.Entry{}, false
	} else {
		return e, true
	}
}

func New(subQS pubsub.QueueSizer, opts ...funcopt.O) *T {
	t := &T{
		log:               plog.NewDefaultLogger().Attr("pkg", "daemon/scheduler").WithPrefix("daemon: scheduler: "),
		localhost:         hostname.Hostname(),
		jobs:              newJobs(),
		now:               time.Now,
		schedules:         make(Schedules),
		failover:          make(map[naming.Path]bool),
		provisioned:       make(map[naming.Path]bool),
		notSource:         make(map[naming.Path]string),
		localStatus:       make(map[naming.Path]instance.Status),
		subQS:             subQS,
		lastRunOnAllPeers: make(timeMap),
		reqSatisfied:      make(errMap),

		status: daemonsubsystem.Scheduler{
			Status:     daemonsubsystem.Status{CreatedAt: time.Now(), ID: "scheduler"},
			MaxRunning: 5,
		},
	}
	t.exec = t.action
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("init: %s", err)
		return nil
	}
	return t
}

func (t *T) peerInstanceLastRun(e schedule.Entry) time.Time {
	if e.Path.IsZero() {
		return time.Time{}
	}
	if !t.isFailover(e.Path) {
		return time.Time{}
	}
	if e.Config.Require == "" {
		return time.Time{}
	}
	if strings.Contains(e.Config.Require, "down") {
		return time.Time{}
	}
	if strings.Contains(e.Config.Require, "warn") {
		return time.Time{}
	}
	lastRunOnAllPeers, ok := t.lastRunOnAllPeers.Get(e.Path, e.Key)
	if !ok {
		return time.Time{}
	}
	return lastRunOnAllPeers
}

func (t *T) jobLogger(e schedule.Entry) *plog.Logger {
	logger := naming.LogWithPath(t.log, e.Path)
	return logger.AddPrefix(e.LogPrefix())
}

func (t *T) isFailover(path naming.Path) bool {
	isFailover, hasFailover := t.failover[path]
	return hasFailover && isFailover
}

func (t *T) isProvisioned(path naming.Path) bool {
	isProvisioned, hasProvisioned := t.provisioned[path]
	return hasProvisioned && isProvisioned
}

// planJob plans the entry to run at the next time its schedule allows after
// its last run, in place of the job planned for its key.
//
// An entry that can not be planned, whose schedule has no next date or can
// not be read, is unplanned rather than left planned for a time that never
// comes: a job planned is a job that will be taken.
func (t *T) planJob(e schedule.Entry, now time.Time) {
	logger := t.jobLogger(e)
	if e.LastRunAt.IsZero() {
		// after daemon start: the last run is read from the last run file
		e.LastRunAt = e.GetLastRun()
	}
	if tm := t.peerInstanceLastRun(e); e.LastRunAt.Before(tm) {
		logger.Infof("adjust schedule entry last run time: %s => %s", e.LastRunAt, tm)
		e.LastRunAt = tm
	}
	next, _, err := e.GetNextAt(now)
	switch {
	case err != nil:
		logger.Warnf("unschedule (failed to find a next date: %s)", err)
		t.jobs.del(e.Path, e.Key)
		return
	case next.IsZero():
		logger.Infof("unschedule (no next date)")
		t.jobs.del(e.Path, e.Key)
		return
	}
	if !e.LastRunAt.IsZero() && !next.After(e.LastRunAt) {
		// A schedule answering its last run as its next one would have
		// the loop take the job again and again without sleeping.
		next = e.LastRunAt.Add(time.Second)
	}
	if next.Before(now) {
		// A date already past is taken as soon as possible, not dropped.
		next = now
	}
	e.NextRunAt = next
	t.jobs.set(e, now)
	logger.Tracef("next at %s (in %s)", next, next.Sub(now))
}

// onTick takes the jobs that are due.
func (t *T) onTick() {
	now := t.now()
	for _, j := range t.jobs.popDue(now) {
		t.onJobDue(j, now)
	}
}

// armTimer sets the timer to wake the loop when the soonest job is due, and
// after maxSleep at most.
func (t *T) armTimer() {
	if t.timer == nil {
		return
	}
	d, ok := t.nextWake(t.now())
	if !ok {
		t.timer.Stop()
		return
	}
	t.timer.Reset(d)
}

// nextWake returns how long the loop may sleep before a job is due, and
// false when no job is planned.
func (t *T) nextWake(now time.Time) (time.Duration, bool) {
	j := t.jobs.first()
	if j == nil {
		return 0, false
	}
	return max(0, min(j.entry.NextRunAt.Sub(now), maxSleep)), true
}

// onJobDue takes a job that is due: it plans the next run of the job, then
// runs it, unless something says to skip this run.
//
// The next run is planned before anything else, so that no reason to skip
// this run leaves the job unplanned: a job skipped because a sync conflicted
// with an orchestration, or because its node was being drained, runs again
// at its next period.
func (t *T) onJobDue(j *job, now time.Time) {
	due := j.entry
	e, ok := t.schedules.Get(due.Path, due.Key)
	if !ok {
		t.jobLogger(due).Infof("unschedule (schedule deleted)")
		t.jobs.del(due.Path, due.Key)
		return
	}
	logger := t.jobLogger(e)

	if tm := t.peerInstanceLastRun(e); due.LastRunAt.Before(tm) {
		logger.Infof("skip (ran on a peer at %s)", tm)
		e.LastRunAt = tm
		t.planJob(e, now)
		t.updateExposedSchedules(e.Path)
		return
	}

	if reason := t.holdReason(e); reason != "" {
		// The action ends in moments, and the job is due: it is tried
		// again then, not at its next period, which for a daily job
		// due during a start would skip a day.
		logger.Infof("hold (%s)", reason)
		e.LastRunAt = due.LastRunAt
		e.NextRunAt = now.Add(holdRetryDelay)
		t.jobs.set(e, now)
		t.updateExposedSchedules(e.Path)
		return
	}

	last := due.NextRunAt
	if now.Sub(last) > lateTolerance {
		last = now
	}
	e.LastRunAt = last
	t.planJob(e, now)
	t.updateExposedSchedules(e.Path)

	if reason := t.skipReason(e); reason != "" {
		logger.Infof("skip (%s)", reason)
		return
	}
	if n, err := t.runningCount(e); err != nil {
		logger.Warnf("skip (%s)", err)
		return
	} else if n >= e.MaxParallel {
		logger.Infof("skip (%d/%d jobs already running)", n, e.MaxParallel)
		return
	}

	// Update the by-node last run cache
	t.lastRunOnAllPeers.Set(e.Path, e.Key, last)

	t.run(e, last, logger)
}

// run runs the job in the background.
//
// The last run is recorded before the run starts, so a daemon restarted
// while the job runs does not run it again at once. The last success is
// recorded once it succeeded.
func (t *T) run(e schedule.Entry, last time.Time, logger *plog.Logger) {
	if err := e.SetLastRun(last); err != nil {
		logger.Errorf("on update last run: %s", err)
	}
	jobRunCount.WithLabelValues(e.Action).Inc()
	jobRunByPathCount.WithLabelValues(e.Action, e.Path.String()).Inc()
	jobRunByPathKeyCount.WithLabelValues(e.Action, e.Path.String(), e.Key).Inc()
	t.running.Add(1)
	go func() {
		defer t.running.Done()
		if err := t.exec(e); err != nil {
			logger.Errorf("on exec: %s", err)
		} else if err := e.SetLastSuccess(last); err != nil {
			// remember last success, for users benefit
			logger.Errorf("on update last success: %s", err)
		}
	}()
}

// blockReason says why the entry can not be scheduled, "" when it can.
//
// These are the conditions the scheduler follows from the events: a job is
// planned when they are met and unplanned when one is not. They are checked
// again when the job is due, to skip a run an event not yet received would
// have unplanned.
func (t *T) blockReason(e schedule.Entry) string {
	if e.RequireCollector && !t.isCollectorJoinable {
		return "collector not joinable"
	}
	if e.Path.IsZero() {
		return ""
	}
	if e.RequireReplicationSource {
		reason, ok := t.notSource[e.Path]
		switch {
		case !ok:
			return "replication source not yet evaluated"
		case reason != "":
			return "not the replication source: " + reason
		}
	}
	if e.RequireProvisioned {
		isProvisioned, ok := t.provisioned[e.Path]
		switch {
		case !ok:
			return "instance provisioned state is still unknown"
		case !isProvisioned:
			return "instance not provisioned"
		}
	}
	if e.Require != "" {
		satisfied, ok := t.reqSatisfied.Get(e.Path, e.Key)
		switch {
		case !ok:
			return fmt.Sprintf("require %s not yet evaluated", e.Require)
		case satisfied != nil:
			return satisfied.Error()
		}
	}
	return ""
}

// holdReason says why a job due now is held back for a moment, "" when it is
// not.
//
// A job whose eligibility is read from the status of the instance, its
// requirements or its provisioned state, is held while an action of the
// instance runs: the status is in flux then. A provision off the leader
// starts the resources and stops them at once, and a task requiring one of
// them up was started in the instant it was, to run on a node it was not to
// run on.
func (t *T) holdReason(e schedule.Entry) string {
	if e.Path.IsZero() || (e.Require == "" && !e.RequireProvisioned) {
		return ""
	}
	mon := instance.MonitorData.GetByPathAndNode(e.Path, t.localhost)
	if mon == nil || !mon.State.IsDoing() {
		return ""
	}
	return fmt.Sprintf("instance %s", mon.State)
}

// skipReason says why the job due now does not run, "" when it runs.
//
// It is blockReason, and the conditions that only last a while and do not
// unplan the job: the next period runs if they are gone.
func (t *T) skipReason(e schedule.Entry) string {
	if reason := t.blockReason(e); reason != "" {
		return reason
	}
	if e.Path.IsZero() || !e.RequireReplicationSource {
		return ""
	}
	// A sync started while the object is being stopped or switched holds
	// the object lock the stop needs, and may be sending when the peer
	// takes over.
	if mon := instance.MonitorData.GetByPathAndNode(e.Path, t.localhost); mon != nil && mon.GlobalExpect != instance.MonitorGlobalExpectNone && mon.GlobalExpect != instance.MonitorGlobalExpectInit {
		return fmt.Sprintf("orchestration %s in progress", mon.GlobalExpect)
	}
	// A node being drained shuts its instances down without waiting for
	// the syncs, and interrupts the ones running: a sync started meanwhile
	// would be sending while a peer takes over.
	if mon := node.MonitorData.GetByNode(t.localhost); mon != nil && mon.LocalExpect == node.MonitorLocalExpectDrained {
		return "node draining"
	}
	return ""
}

func (t *T) runningCount(e schedule.Entry) (int, error) {
	if e.RunDir == "" {
		return -1, nil
	}
	dir := runfiles.Dir{
		Path: e.RunDir,
		Log:  t.jobLogger(e),
	}
	n, err := dir.Count()
	if err != nil {
		return -1, err
	}
	return n, nil
}

func (t *T) Start(ctx context.Context) error {
	errC := make(chan error)
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		errC <- nil
		t.loop()
	}(errC)

	return <-errC
}

func (t *T) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) startSubscriptions() *pubsub.Subscription {
	sub := pubsub.SubFromContext(t.ctx, "daemon.scheduler", t.subQS)
	labelLocalhost := pubsub.Label{"node", t.localhost}
	sub.AddFilter(&msgbus.AuditStart{}, labelLocalhost)
	sub.AddFilter(&msgbus.AuditStop{}, labelLocalhost)
	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, labelLocalhost)
	sub.AddFilter(&msgbus.ObjectStatusDeleted{}, labelLocalhost)
	sub.AddFilter(&msgbus.ObjectStatusUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeMonitorUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.DaemonCollectorUpdated{}, labelLocalhost)
	sub.Start()
	return sub
}

func (t *T) loop() {
	t.log.Tracef("loop started")
	t.databus = daemondata.FromContext(t.ctx)
	t.publisher = pubsub.PubFromContext(t.ctx)
	sub := t.startSubscriptions()

	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	t.timer = time.NewTimer(maxSleep)
	defer t.timer.Stop()

	// The NodeMonitorUpdated event can be fired before our subscription.
	// As this event enables the scheduler, we can't afford missing it.
	// Read the NodeMonitor state from cache.
	if nodeMonitorData := node.MonitorData.GetByNode(t.localhost); nodeMonitorData != nil {
		t.toggleEnabled(nodeMonitorData.State)
	}

	t.status.State = "running"
	t.status.ConfiguredAt = time.Now()
	if nodeConfig := node.ConfigData.GetByNode(hostname.Hostname()); nodeConfig != nil {
		if nodeConfig.MaxParallel > 0 {
			t.maxRunning = nodeConfig.MaxParallel
			t.status.MaxRunning = t.maxRunning
			t.publishUpdate()
		} else {
			t.log.Warnf("ignore node config with MaxParallel value 0")
		}
	}

	// The jobs planned above, at start, are taken when due, not at the
	// first wake of a timer set before they were planned.
	t.armTimer()

	for {
		select {
		case ev := <-sub.C:
			switch c := ev.(type) {
			case *msgbus.AuditStart:
				t.log.HandleAuditStart(c.Q, c.Subsystems, "scheduler")
			case *msgbus.AuditStop:
				t.log.HandleAuditStop(c.Q, c.Subsystems, "scheduler")
			case *msgbus.InstanceStatusDeleted:
				t.onInstanceStatusDeleted(c)
			case *msgbus.NodeMonitorUpdated:
				t.onNodeMonitorUpdated(c)
			case *msgbus.NodeConfigUpdated:
				t.onNodeConfigUpdated(c)
			case *msgbus.ObjectStatusUpdated:
				t.onObjectStatusUpdated(c)
			case *msgbus.ObjectStatusDeleted:
				t.onObjectStatusDeleted(c)
			case *msgbus.DaemonCollectorUpdated:
				t.onDaemonCollectorUpdated(c)
			}
		case <-t.timer.C:
			t.onTick()
		case <-t.ctx.Done():
			t.jobs.purge()
			return
		}
		// Whatever was handled may have planned a job sooner than the
		// timer is set to wake, or unplanned the one it waits for.
		t.armTimer()
	}
}

func (t *T) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	t.loggerWithPath(c.Path).Infof("unschedule all jobs (instance deleted)")
	t.unschedule(c.Path)
}

func (t *T) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) bool {
	if c.Node == t.localhost {
		return t.onLocalInstanceStatusUpdated(c)
	} else {
		return t.onPeerInstanceStatusUpdated(c)
	}
}

func (t *T) onLocalInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) bool {
	// The status is kept whether or not the schedules of the object are
	// known yet: at daemon start, the first status comes before them, and
	// scheduleObject evaluates the requirements of the jobs from it.
	t.localStatus[c.Path] = c.Value
	changed := t.updateNotSource(c.Path, c.Value)

	schedules, ok := t.schedules[c.Path]
	if !ok {
		return changed
	}

	t.lastRunOnAllPeers.Set(c.Path, kwoption.ScheduleStatus, c.Value.UpdatedAt)

	for _, e := range schedules {
		if e.Require == "" {
			continue
		}
		if t.updateReqSatisfied(c.Path, e, c.Value) {
			changed = true
		}
	}
	return changed
}

// checkRequire says why the requirement of a job is not met by the instance
// status st, nil when all its conditions are.
func checkRequire(require string, st instance.Status) error {
	for rid, requiredStatusList := range resourcereqs.New(require).Requirements() {
		resourceStatus, ok := st.Resources[rid]
		if !ok {
			return fmt.Errorf("resource %s not found in the instance status data", rid)
		}
		if !requiredStatusList.Has(resourceStatus.Status) {
			return fmt.Errorf("resource %s status is %s, required %s", rid, resourceStatus.Status, requiredStatusList)
		}
	}
	return nil
}

// updateReqSatisfied records whether the requirement of the job e is met by
// the instance status st, and reports whether that changed.
func (t *T) updateReqSatisfied(path naming.Path, e schedule.Entry, st instance.Status) bool {
	log := t.jobLogger(e)
	satisfied := checkRequire(e.Require, st)
	currentlySatisfied, ok := t.reqSatisfied.Get(path, e.Key)
	t.reqSatisfied.Set(path, e.Key, satisfied)
	switch {
	case satisfied != nil && !ok:
		log.Tracef("requirement unsatisfied: %s", satisfied)
	case satisfied != nil && currentlySatisfied == nil:
		log.Tracef("requirement no longer satisfied: %s", satisfied)
	case satisfied == nil && !ok:
		log.Tracef("requirement satisfied: %s", e.Require)
	case satisfied == nil && currentlySatisfied != nil:
		log.Tracef("requirement now satisfied: %s", e.Require)
	default:
		return false
	}
	return true
}

func (t *T) onPeerInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) bool {
	if _, ok := t.schedules[c.Path]; !ok {
		// we don't have a local instance
		return false
	}
	pathLog := t.loggerWithPath(c.Path)
	for rid, r := range c.Value.Resources {
		log := pathLog.AddPrefix(rid + ": ")
		resourceId, err := resourceid.Parse(rid)
		if err != nil {
			continue
		}
		switch resourceId.DriverGroup() {
		case driver.GroupTask:
		case driver.GroupSync:
		default:
			continue
		}
		if _, ok := t.lastRunOnAllPeers.Get(c.Path, ridScheduleKey(rid)); !ok {
			if tm, nodename, err := t.readLastRunOnFile(c.Path, rid); err == nil {
				log.Infof("initialize last run at %s on %s", tm, nodename)
				t.lastRunOnAllPeers.Set(c.Path, ridScheduleKey(rid), tm)
			}
		}
		i, ok := r.Info["last_run_at"]
		if !ok {
			continue
		}
		var lastRunAtOnPeer time.Time
		switch v := i.(type) {
		case time.Time:
			lastRunAtOnPeer = v
		case string:
			tm, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				continue
			}
			lastRunAtOnPeer = tm
		}
		if !ok {
			continue
		}
		if err := t.updateLastRunOnFile(c.Path, rid, c.Node, lastRunAtOnPeer); err != nil {
			log.Warnf("write last run on file: %s", err)
		}

		lastestRunAtOnPeer, ok := t.lastRunOnAllPeers.Get(c.Path, ridScheduleKey(rid))

		if !ok || lastRunAtOnPeer.After(lastestRunAtOnPeer) {
			log.Tracef("last run on peer %s at %s", c.Node, lastRunAtOnPeer)
			t.lastRunOnAllPeers.Set(c.Path, ridScheduleKey(rid), lastRunAtOnPeer)
		}
	}
	return false
}

func (t *T) lastRunOnFile(path naming.Path, rid string) string {
	return filepath.Join(path.VarDir(), rid, "last_run_on")
}

func (t *T) lastRunOnFileModTime(path naming.Path, rid string) (time.Time, error) {
	p := t.lastRunOnFile(path, rid)
	if stat, err := os.Stat(p); err != nil {
		return time.Time{}, err
	} else {
		return stat.ModTime(), nil
	}
}

func (t *T) readLastRunOnFile(path naming.Path, rid string) (time.Time, string, error) {
	tm, err := t.lastRunOnFileModTime(path, rid)
	if err != nil {
		return tm, "", err
	}
	p := t.lastRunOnFile(path, rid)
	b, err := os.ReadFile(p)
	if err != nil {
		return tm, "", err
	}
	nodename := strings.TrimSpace(string(b))
	return tm, nodename, nil
}

func (t *T) updateLastRunOnFile(path naming.Path, rid, nodename string, tm time.Time) error {
	lastTm, err := t.lastRunOnFileModTime(path, rid)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(t.lastRunOnFile(path, rid)), 0755); err != nil {
			return err
		}
		return t.writeLastRunOnFile(path, rid, nodename, tm)
	} else if err != nil {
		return err
	}
	if !lastTm.IsZero() && lastTm.After(tm) {
		return nil
	}
	return t.writeLastRunOnFile(path, rid, nodename, tm)
}

func (t *T) writeLastRunOnFile(path naming.Path, rid, nodename string, tm time.Time) error {
	p := t.lastRunOnFile(path, rid)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s\n", nodename); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chtimes(p, tm, tm)
}

func (t *T) onDaemonCollectorUpdated(c *msgbus.DaemonCollectorUpdated) {
	previousIsCollectorJoinable := t.isCollectorJoinable
	switch c.Value.State {
	case "speaker", "speaker-candidate":
		t.isCollectorJoinable = true
		if !previousIsCollectorJoinable {
			t.log.Infof("enable jobs requiring a joinable collector")
			t.scheduleAll()
		}
	default:
		t.isCollectorJoinable = false
		if previousIsCollectorJoinable {
			t.log.Infof("disable jobs requiring a joinable collector")
			t.scheduleAll()
		}
	}
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.lastRunOnAllPeers.UnsetPath(c.Path)
	t.reqSatisfied.UnsetPath(c.Path)
	delete(t.provisioned, c.Path)
	delete(t.failover, c.Path)
	delete(t.notSource, c.Path)
	delete(t.localStatus, c.Path)
}

func (t *T) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if c.Value.ActorStatus == nil {
		return
	}
	var changed bool
	switch srcEv := c.SrcEv.(type) {
	case *msgbus.InstanceStatusUpdated:
		if t.onInstanceStatusUpdated(srcEv) {
			changed = true
		}
	case *msgbus.InstanceConfigUpdated:
		if t.onInstanceConfigUpdated(srcEv) {
			changed = true
		}
	}
	if c.Value.Provisioned == provisioned.Undef {
		delete(t.provisioned, c.Path)
		return
	}
	if t.updateFailover(c.Path, c.Value.Topology) {
		changed = true
	}
	if t.updateProvisioned(c.Path, c.Value.Provisioned) {
		changed = true
	}
	if changed {
		t.scheduleObject(c.Path)
	}
}

func (t *T) updateFailover(path naming.Path, state topology.T) bool {
	wasFailover, ok := t.failover[path]
	isFailover := state == topology.Failover
	t.failover[path] = isFailover
	if !ok || isFailover != wasFailover {
		return true
	}
	return false
}

// updateNotSource records whether the local instance of path is the one its
// data is replicated from, and reports whether that changed.
func (t *T) updateNotSource(path naming.Path, st instance.Status) bool {
	isSource, reason := st.ReplicationSource()
	if isSource {
		reason = ""
	}
	was, ok := t.notSource[path]
	t.notSource[path] = reason
	return !ok || (was == "") != (reason == "")
}

func (t *T) updateProvisioned(path naming.Path, state provisioned.T) bool {
	isProvisioned := state.IsOneOf(provisioned.True, provisioned.NotApplicable)
	wasProvisioned, ok := t.provisioned[path]
	t.provisioned[path] = isProvisioned
	if !ok || isProvisioned != wasProvisioned {
		return true
	}
	return false
}

func (t *T) loggerWithPath(path naming.Path) *plog.Logger {
	return naming.LogWithPath(t.log, path).AddPrefix(fmt.Sprintf("%s: ", path))
}

func (t *T) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) bool {
	if c.Value.ActorConfig == nil {
		t.loggerWithPath(c.Path).Tracef("ignore config change: not actor")
		return false
	}
	if c.Node != t.localhost {
		t.loggerWithPath(c.Path).Tracef("ignore config change: config update event is from a peer")
		return false
	}
	if !t.enabled {
		t.loggerWithPath(c.Path).Tracef("ignore config change: scheduler is disabled")
		return false
	}
	t.loggerWithPath(c.Path).Tracef("update schedules on config change")
	return true
}

func (t *T) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	if c.Value.MaxParallel > 0 {
		t.maxRunning = c.Value.MaxParallel

		if t.status.MaxRunning != t.maxRunning {
			t.log.Infof("max running changed %d -> %d", t.status.MaxRunning, t.maxRunning)
			t.status.MaxRunning = t.maxRunning
			t.publishUpdate()
		}
	} else {
		t.log.Warnf("on NodeConfigUpdated ignore MaxParallel value 0")
	}
	switch {
	case t.enabled:
		t.log.Tracef("node: update schedules on config change")
		t.scheduleNode()
	}
}

func (t *T) onNodeMonitorUpdated(c *msgbus.NodeMonitorUpdated) {
	t.toggleEnabled(c.Value.State)
}

func (t *T) isNodeStateCompatible(state node.MonitorState) bool {
	_, ok := incompatibleNodeMonitorStatus[state]
	return !ok
}

func (t *T) toggleEnabled(state node.MonitorState) {
	isNodeStateCompatible := t.isNodeStateCompatible(state)
	switch {
	case !isNodeStateCompatible && t.enabled:
		t.log.Infof("disable scheduling (node monitor status is now %s)", state)
		t.jobs.purge()
		t.enabled = false
	case isNodeStateCompatible && !t.enabled:
		t.log.Infof("enable scheduling (node monitor status is now %s)", state)
		t.enabled = true
		t.scheduleAll()
	}
}

func (t *T) scheduleAll() {
	for p := range instance.StatusData.GetByNode(t.localhost) {
		t.scheduleObject(p)
	}
	t.scheduleNode()
}

func (t *T) scheduleNode() {
	if !t.enabled {
		return
	}
	nodeConfig := node.ConfigData.GetByNode(t.localhost)
	if nodeConfig == nil {
		return
	}
	t.scheduleEntries(naming.Path{}, nodeConfig.Schedules)
}

func (t *T) scheduleObject(path naming.Path) {
	if !t.enabled {
		return
	}
	instanceConfig := instance.ConfigData.GetByPathAndNode(path, t.localhost)
	if instanceConfig == nil || instanceConfig.ActorConfig == nil {
		// only actor objects have scheduled actions
		return
	}
	t.scheduleEntries(path, instanceConfig.Schedules)
}

// scheduleEntries plans the jobs of the schedules of the node or an object,
// the ones not planned yet and the ones whose schedule changed, and unplans
// the ones that can no longer run and the ones no longer configured.
func (t *T) scheduleEntries(path naming.Path, configs []schedule.Config) {
	now := t.now()
	seen := make(map[string]bool, len(configs))
	for _, config := range configs {
		e := schedule.Entry{
			Node:   t.localhost,
			Path:   path,
			Config: config,
		}
		seen[e.Key] = true
		prev, hadSchedule := t.schedules.Get(path, e.Key)
		t.schedules.Add(path, e)
		j, hasJob := t.jobs.get(path, e.Key)
		logger := t.jobLogger(e)
		unschedule := func(reason string) {
			if hasJob {
				logger.Infof("unschedule (%s)", reason)
				t.jobs.del(path, e.Key)
			}
		}

		if e.Schedule == "" || e.Schedule == "@0" {
			unschedule("schedule is @0")
			continue
		}

		// The requirements not evaluated yet are evaluated from the last
		// status of the instance, when one came before the schedules.
		if st, ok := t.localStatus[path]; ok && !path.IsZero() {
			if _, ok := t.notSource[path]; !ok && e.RequireReplicationSource {
				t.updateNotSource(path, st)
			}
			if _, ok := t.reqSatisfied.Get(path, e.Key); !ok && e.Require != "" {
				t.updateReqSatisfied(path, e, st)
			}
		}
		if reason := t.blockReason(e); reason != "" {
			unschedule(reason)
			continue
		}

		switch {
		case !hasJob:
			t.planJob(e, now)
			if j, ok := t.jobs.get(path, e.Key); ok {
				logger.Infof("schedule (next at %s)", j.entry.NextRunAt.Format(time.RFC3339))
			}
		case hadSchedule && prev.Schedule != e.Schedule:
			// The job keeps its last run: the new schedule counts from it.
			e.LastRunAt = j.entry.LastRunAt
			t.planJob(e, now)
			if j, ok := t.jobs.get(path, e.Key); ok {
				logger.Infof("reschedule (schedule changed from %s, next at %s)", prev.Schedule, j.entry.NextRunAt.Format(time.RFC3339))
			}
		}
	}

	// Unplan the jobs, and forget the schedules, no longer configured.
	for _, key := range t.jobs.keys(path) {
		if !seen[key] {
			j, _ := t.jobs.get(path, key)
			t.jobLogger(j.entry).Infof("unschedule (no longer configured)")
			t.jobs.del(path, key)
		}
	}
	for key := range t.schedules[path] {
		if !seen[key] {
			t.schedules.Del(path, key)
		}
	}

	t.updateExposedSchedules(path)
}

func (t *T) updateExposedSchedules(path naming.Path) {
	table := t.schedules.Table(path)
	if table == nil {
		return
	}
	table = table.Merge(t.jobs.table(path))
	schedule.TableData.Set(path, &table)
}

func (t *T) unschedule(path naming.Path) {
	t.reqSatisfied.UnsetPath(path)
	delete(t.notSource, path)
	delete(t.localStatus, path)
	t.schedules.DelPath(path)
	t.jobs.delPath(path)
	schedule.TableData.Unset(path)
}

func (t *T) publishUpdate() {
	t.status.UpdatedAt = time.Now()
	daemonsubsystem.DataScheduler.Set(t.localhost, t.status.DeepCopy())
	t.publisher.Pub(&msgbus.DaemonSchedulerUpdated{Node: t.localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", t.localhost})
}

func (t pathKeyMap[V]) Get(path naming.Path, key string) (V, bool) {
	v, ok := t[path][key]
	return v, ok
}

func (t pathKeyMap[V]) Set(path naming.Path, key string, v V) {
	m, ok := t[path]
	if !ok {
		m = make(map[string]V)
		t[path] = m
	}
	m[key] = v
}

func (t pathKeyMap[V]) Unset(path naming.Path, key string) {
	delete(t[path], key)
	if len(t[path]) == 0 {
		delete(t, path)
	}
}

func (t pathKeyMap[V]) UnsetPath(path naming.Path) {
	delete(t, path)
}

// ridScheduleKey is the key of the last run of the schedule of a resource.
func ridScheduleKey(rid string) string {
	return rid + ".schedule"
}
