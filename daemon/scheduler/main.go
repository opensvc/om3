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
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
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

		jobs                Jobs
		events              chan any
		enabled             bool
		provisioned         map[naming.Path]bool
		failover            map[naming.Path]bool
		schedules           Schedules
		isCollectorJoinable bool

		wg sync.WaitGroup

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

	// Job is a schedule entries with alarms set.
	Job struct {
		CreatedAt time.Time
		LastRunAt time.Time
		schedule  schedule.Entry
		cancel    []func()
	}

	// Jobs is a map of Job
	Jobs map[string]Job

	eventJobAlarm struct {
		schedule schedule.Entry
	}
	eventJobDone struct {
		schedule schedule.Entry
		end      time.Time
	}

	timeMap map[string]time.Time
	errMap  map[string]error
)

var (
	incompatibleNodeMonitorStatus = map[node.MonitorState]any{
		node.MonitorStateInit:             nil,
		node.MonitorStateMaintenance:      nil,
		node.MonitorStateRejoin:           nil,
		node.MonitorStateShutdownProgress: nil,
		node.MonitorStateUpgrade:          nil,
	}

	jobRunByPathKeyCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "scheduler",
			Name:      "object_job_runs_total",
		}, []string{"action", "path", "key"})

	jobRunByPathCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "scheduler",
			Name:      "object_runs_total",
		}, []string{"action", "path"})

	jobRunCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "opensvc",
			Subsystem: "scheduler",
			Name:      "runs_total",
		}, []string{"action"})
)

func (t Schedules) Del(path naming.Path) {
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
		events:            make(chan any),
		jobs:              make(Jobs),
		schedules:         make(Schedules),
		failover:          make(map[naming.Path]bool),
		provisioned:       make(map[naming.Path]bool),
		subQS:             subQS,
		lastRunOnAllPeers: make(timeMap),
		reqSatisfied:      make(errMap),

		status: daemonsubsystem.Scheduler{
			Status:     daemonsubsystem.Status{CreatedAt: time.Now(), ID: "scheduler"},
			MaxRunning: 5,
		},
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("init: %s", err)
		return nil
	}
	return t
}

func newJobId(e schedule.Entry) string {
	return fmt.Sprintf("%s:%s", e.Path, e.Key)
}

func (t Jobs) PathIds(path naming.Path) []string {
	var l []string
	for id, job := range t {
		if job.schedule.Path == path {
			l = append(l, id)
		}
	}
	return l
}

func (t Jobs) Table(path naming.Path) schedule.Table {
	table := make(schedule.Table, 0)
	for _, job := range t {
		if job.schedule.Path == path {
			table = append(table, job.schedule)
		}
	}
	return table
}

func (t Jobs) Add(e schedule.Entry, delay time.Duration, bus chan any) {
	tmr := time.AfterFunc(delay, func() {
		bus <- eventJobAlarm{
			schedule: e,
		}
	})
	cancel := func() {
		if tmr == nil {
			return
		}
		tmr.Stop()
	}
	jobId := newJobId(e)
	job, ok := t[jobId]
	if !ok {
		job = Job{
			CreatedAt: time.Now(),
		}
	}
	job.schedule = e
	job.cancel = append(job.cancel, cancel)
	t[jobId] = job
}

func (t Jobs) Done(e schedule.Entry) Job {
	jobId := newJobId(e)
	job, ok := t[jobId]
	if !ok {
		return Job{}
	}
	job.Cancel()
	t[jobId] = job
	return job
}

func (t Jobs) DelId(jobId string) {
	job, ok := t[jobId]
	if !ok {
		return
	}
	job.Cancel()
	delete(t, jobId)
}

func (t Jobs) Del(e schedule.Entry) {
	jobId := newJobId(e)
	t.DelId(jobId)
}

func (t Jobs) DelPath(p naming.Path) {
	for _, e := range t {
		if e.schedule.Path != p {
			continue
		}
		t.Del(e.schedule)
	}
}

func (t Jobs) Purge() {
	for k, e := range t {
		e.Cancel()
		delete(t, k)
	}
}

func (t Jobs) Has(e schedule.Entry) bool {
	jobId := newJobId(e)
	_, ok := t[jobId]
	return ok
}

func (t Jobs) Get(e schedule.Entry) (Job, bool) {
	jobId := newJobId(e)
	job, ok := t[jobId]
	return job, ok
}

func (t Job) Cancel() {
	for _, cancel := range t.cancel {
		cancel()
	}
	t.cancel = nil
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

func (t *T) createJob(e schedule.Entry) {
	if !t.enabled {
		return
	}
	if e.RequireCollector && !t.isCollectorJoinable {
		return
	}
	if e.Require != "" {
		if satisfied, ok := t.reqSatisfied.Get(e.Path, e.Key); !ok || satisfied != nil {
			return
		}
	}

	logger := t.jobLogger(e)
	if e.LastRunAt.IsZero() {
		// after daemon start: initialize the schedule's LastRunAt from LastRunFile
		e.LastRunAt = e.GetLastRun()
	}

	if tm := t.peerInstanceLastRun(e); e.LastRunAt.Before(tm) {
		logger.Infof("adjust schedule entry last run time: %s => %s", e.LastRunAt, tm)
		e.LastRunAt = tm
	}

	now := time.Now() // keep before GetNext call
	next, _, err := e.GetNext()
	if err != nil {
		logger.Warnf("failed to find a next date: %s", err)
		t.jobs.Del(e)
		return
	}
	if next.IsZero() {
		t.jobs.Del(e)
		return
	}
	if next.Before(now) {
		logger.Warnf("next %s is in the past", next)
		t.jobs.Del(e)
		return
	}
	e.NextRunAt = next
	delay := next.Sub(now)
	if e.LastRunAt.IsZero() || delay >= time.Second {
		logger.Tracef("next at %s (in %s)", next, delay)
	}
	t.jobs.Add(e, delay, t.events)
	return
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

func (t *T) onJobAlarm(c eventJobAlarm) {
	logger := t.jobLogger(c.schedule)
	e, ok := t.schedules.Get(c.schedule.Path, c.schedule.Key)
	if !ok {
		logger.Infof("abort (schedule deleted)")
		return
	}
	if e.RequireCollector && !t.isCollectorJoinable {
		logger.Infof("abort (collector not joinable)")
		return
	}
	if !e.Path.IsZero() {
		if e.RequireProvisioned && !t.isProvisioned(e.Path) {
			logger.Infof("abort (no longer provisioned)")
			return
		}
		if satisfied, ok := t.reqSatisfied.Get(e.Path, e.Key); ok {
			if satisfied != nil {
				logger.Infof("abort (requirements no longer met)")
				return
			}
		} else if e.Require != "" {
			logger.Infof("abort (requirements not yet evaluated)")
			return
		}
	}

	if tm := t.peerInstanceLastRun(e); c.schedule.LastRunAt.Before(tm) {
		logger.Infof("abort (job ran on peer at %s)", tm)
		t.recreateJobFrom(e, tm)
		return
	}

	// plan the next run before exec, so another exec can be done
	// even if another is running
	e.LastRunAt = c.schedule.LastRunAt
	e.NextRunAt = c.schedule.NextRunAt
	t.recreateJobFrom(e, c.schedule.NextRunAt)

	if n, err := t.runningCount(e); err != nil {
		logger.Warnf("%s", err)
		return
	} else if n >= e.MaxParallel {
		logger.Infof("abort (%d/%d jobs already running)", n, e.MaxParallel)
		return
	}

	// Update the by-node last run cache
	t.lastRunOnAllPeers.Set(e.Path, e.Key, e.NextRunAt)

	// Update the job last run date
	jobId := newJobId(e)
	job, ok := t.jobs[jobId]
	if ok {
		job.LastRunAt = c.schedule.NextRunAt
		t.jobs[jobId] = job
	}

	go func() {
		jobRunCount.WithLabelValues(e.Action).Inc()
		jobRunByPathCount.WithLabelValues(e.Action, e.Path.String()).Inc()
		jobRunByPathKeyCount.WithLabelValues(e.Action, e.Path.String(), e.Key).Inc()
		if err := t.action(e); err != nil {
			logger.Errorf("on exec: %s", err)
		} else {
			// remember last success, for users benefit
			if err := e.SetLastSuccess(c.schedule.NextRunAt); err != nil {
				logger.Errorf("on update last success: %s", err)
			}
		}

		// remember last run, to not run the job too soon after a daemon restart
		if err := e.SetLastRun(c.schedule.NextRunAt); err != nil {
			logger.Errorf("on update last run: %s", err)
		}

		t.events <- eventJobDone{
			schedule: e,
			end:      time.Now(),
		}
	}()
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
		case ev := <-t.events:
			switch c := ev.(type) {
			case eventJobAlarm:
				t.onJobAlarm(c)
			case eventJobDone:
				t.onJobDone(c)
			default:
				t.log.Errorf("received an unsupported event: %#v", c)
			}
		case <-t.ctx.Done():
			t.jobs.Purge()
			return
		}
	}
}

func (t *T) onJobDone(c eventJobDone) {
	job := t.jobs.Done(c.schedule)
	t.recreateJobFrom(c.schedule, job.LastRunAt)
}

func (t *T) recreateJobFrom(prev schedule.Entry, lastRunAt time.Time) {
	e, ok := t.schedules.Get(prev.Path, prev.Key)
	if !ok {
		// no longer scheduled
		return
	}
	e.LastRunAt = lastRunAt
	t.createJob(e)
	t.updateExposedSchedules(prev.Path)
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
	schedules, ok := t.schedules[c.Path]
	if !ok {
		return false
	}

	t.lastRunOnAllPeers.Set(c.Path, kwoption.ScheduleStatus, c.Value.UpdatedAt)

	checkReq := func(rid string, requiredStatusList status.L) error {
		resourceStatus, ok := c.Value.Resources[rid]
		if !ok {
			return fmt.Errorf("resource %s not found in the instance status data", rid)
		}
		if !requiredStatusList.Has(resourceStatus.Status) {
			return fmt.Errorf("resource %s status is %s, required %s", rid, resourceStatus.Status, requiredStatusList)
		}
		return nil
	}

	changed := false

	for _, e := range schedules {
		if e.Require == "" {
			continue
		}
		log := t.jobLogger(e)
		reqs := resourcereqs.New(e.Require)
		for requiredRID, requiredStatusList := range reqs.Requirements() {
			satisfied := checkReq(requiredRID, requiredStatusList)
			currentlySatisfied, ok := t.reqSatisfied.Get(c.Path, e.Key)
			t.reqSatisfied.Set(c.Path, e.Key, satisfied)
			if satisfied != nil {
				if !ok {
					log.Tracef("requirement unsatisfied: %s", satisfied)
					changed = true
				} else if currentlySatisfied == nil {
					log.Tracef("requirement no longer satisfied: %s", satisfied)
					changed = true
				}
			} else {
				if !ok {
					log.Tracef("requirement satisfied: %s", e.Require)
					changed = true
				} else if currentlySatisfied != nil {
					log.Tracef("requirement now satisfied: %s", e.Require)
					changed = true
				}
			}
		}
	}
	return changed
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
		if _, ok := t.lastRunOnAllPeers.GetWithRID(c.Path, rid); !ok {
			if tm, nodename, err := t.readLastRunOnFile(c.Path, rid); err == nil {
				log.Infof("initialize last run at %s on %s", tm, nodename)
				t.lastRunOnAllPeers.SetWithRID(c.Path, rid, tm)
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

		lastestRunAtOnPeer, ok := t.lastRunOnAllPeers.GetWithRID(c.Path, rid)

		if !ok || lastRunAtOnPeer.After(lastestRunAtOnPeer) {
			log.Tracef("last run on peer %s at %s", c.Node, lastRunAtOnPeer)
			t.lastRunOnAllPeers.SetWithRID(c.Path, rid, lastRunAtOnPeer)
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
		return os.MkdirAll(filepath.Dir(t.lastRunOnFile(path, rid)), 755)
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
	defer f.Close()
	fmt.Fprintf(f, "%s\n", nodename)
	os.Chtimes(p, tm, tm)
	return nil
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
			t.unscheduleRequireCollector()
			t.scheduleAll()
		}
	}
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.lastRunOnAllPeers.UnsetPath(c.Path)
	t.reqSatisfied.UnsetPath(c.Path)
	delete(t.provisioned, c.Path)
	delete(t.failover, c.Path)
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
		t.jobs.Purge()
		t.enabled = false
	case isNodeStateCompatible && !t.enabled:
		t.log.Infof("enable scheduling (node monitor status is now %s)", state)
		t.enabled = true
		t.scheduleAll()
	}
}

func (t *T) hasAnyJob(p naming.Path) bool {
	for _, job := range t.jobs {
		if job.schedule.Path == p {
			return true
		}
	}
	return false
}

func (t *T) scheduleAll() {
	for p, _ := range instance.StatusData.GetByNode(t.localhost) {
		t.scheduleObject(p)
	}
	t.scheduleNode()
}

func (t *T) scheduleNode() {
	if !t.enabled {
		return
	}
	nodeConfig := node.ConfigData.GetByNode(t.localhost)
	if nodeConfig == nil || nodeConfig.Schedules == nil || len(nodeConfig.Schedules) == 0 {
		return
	}

	path := naming.Path{}
	jobIds := make(map[string]any)

	scheduleOne := func(scheduleConfig schedule.Config) {
		e := schedule.Entry{
			Node:   t.localhost,
			Config: scheduleConfig,
		}

		// Remember we saw that job id, so we can purge unconfigured jobs after this loop
		jobIds[newJobId(e)] = nil

		log := t.jobLogger(e)
		prevSchedule, hasSchedule := t.schedules.Get(path, e.Key)
		hasJob := t.jobs.Has(e)
		t.schedules.Add(path, e)
		var validated []string
		var skipped string

		if hasSchedule && (scheduleConfig.Schedule != prevSchedule.Config.Schedule) {
			if hasJob {
				if e.Schedule == "" || e.Schedule == "@0" {
					log.Infof("unschedule (schedule is now @0)")
					t.jobs.Del(e)
					return
				} else {
					// At the end of this func, if we did not recreate the job,
					// log it as unscheduled
					defer func() {
						if !t.jobs.Has(e) {
							log.Infof("unschedule on config change, skip reschedule (%s)", skipped)
						}
					}()
					t.jobs.Del(e)
				}
			} else {
				if e.Schedule == "" || e.Schedule == "@0" {
					return
				}
			}
		} else if e.Schedule == "" || e.Schedule == "@0" {
			return
		}
		validated = append(validated, e.Schedule)

		if e.RequireCollector && !t.isCollectorJoinable {
			if hasJob {
				log.Infof("unschedule (collector unjoignable)")
				t.jobs.Del(e)
			} else {
				skipped = "collector unjoignable"
			}
			return
		}

		if !t.jobs.Has(e) {
			log.Infof("schedule (%s)", strings.Join(validated, ", "))
			t.createJob(e)
		}
	}

	for _, scheduleConfig := range nodeConfig.Schedules {
		scheduleOne(scheduleConfig)
	}

	// Purge unconfigured jobs
	for _, id := range t.jobs.PathIds(path) {
		if _, ok := jobIds[id]; !ok {
			job := t.jobs[id]
			t.jobLogger(job.schedule).Infof("unschedule (no longer configured)")
			t.jobs.DelId(id)
		}
	}

	t.updateExposedSchedules(path)
}

func (t *T) scheduleObject(path naming.Path) {
	if !t.enabled {
		return
	}

	instanceConfig := instance.ConfigData.GetByPathAndNode(path, t.localhost)
	if instanceConfig == nil || instanceConfig.ActorConfig == nil || instanceConfig.Schedules == nil || len(instanceConfig.Schedules) == 0 {
		// only actor objects have scheduled actions
		return
	}

	isProvisioned, hasProvisioned := t.provisioned[path]
	jobIds := make(map[string]any)

	scheduleOne := func(scheduleConfig schedule.Config) {
		e := schedule.Entry{
			Node:   t.localhost,
			Path:   path,
			Config: scheduleConfig,
		}

		// Remember we saw that job id, so we can purge unconfigured jobs after this loop
		jobIds[newJobId(e)] = nil

		log := t.jobLogger(e)
		prevSchedule, hasSchedule := t.schedules.Get(path, e.Key)
		hasJob := t.jobs.Has(e)
		t.schedules.Add(path, e)
		var validated []string
		var skipped string

		if hasSchedule && (scheduleConfig.Schedule != prevSchedule.Config.Schedule) {
			if hasJob {
				if e.Schedule == "" || e.Schedule == "@0" {
					log.Infof("unschedule (schedule is now @0)")
					t.jobs.Del(e)
					return
				} else {
					// At the end of this func, if we did not recreate the job,
					// log it as unscheduled
					defer func() {
						if !t.jobs.Has(e) {
							log.Infof("unschedule on config change, skip reschedule (%s)", skipped)
						}
					}()
					t.jobs.Del(e)
				}
			} else {
				if e.Schedule == "" || e.Schedule == "@0" {
					return
				}
			}
		} else if e.Schedule == "" || e.Schedule == "@0" {
			return
		}
		validated = append(validated, e.Schedule)

		if e.RequireCollector && !t.isCollectorJoinable {
			if hasJob {
				log.Infof("unschedule (collector unjoignable)")
				t.jobs.Del(e)
			} else {
				skipped = "collector unjoignable"
			}
			return
		}

		if e.RequireProvisioned {
			if !hasProvisioned {
				if hasJob {
					log.Infof("unschedule (instance provisioned state is still unknown)")
					t.jobs.Del(e)
				} else {
					skipped = "instance provisioned state is still unknown"
				}
				return
			}
			if !isProvisioned {
				if hasJob {
					log.Infof("unschedule (instance no longer provisionned)")
					t.jobs.Del(e)
				} else {
					skipped = "instance no longer provisionned"
				}
				return
			}
			validated = append(validated, "provisioned")
		}
		if satisfied, ok := t.reqSatisfied.Get(path, e.Key); ok {
			if satisfied != nil {
				if hasJob {
					log.Infof("unschedule (%s)", satisfied)
					t.jobs.Del(e)
				} else {
					skipped = satisfied.Error()
				}
				return
			}
			validated = append(validated, fmt.Sprintf("require %s is satisfied", e.Require))
		} else if e.Require != "" {
			if hasJob {
				log.Infof("unschedule (require %s not yet evaluated)", e.Require)
				t.jobs.Del(e)
			} else {
				skipped = fmt.Sprintf("require %s not yet evaluated", e.Require)
			}
			return
		}
		if !t.jobs.Has(e) {
			log.Infof("schedule (%s)", strings.Join(validated, ", "))
			t.createJob(e)
		}
	}

	for _, scheduleConfig := range instanceConfig.Schedules {
		scheduleOne(scheduleConfig)
	}

	// Purge unconfigured jobs
	for _, id := range t.jobs.PathIds(path) {
		if _, ok := jobIds[id]; !ok {
			job := t.jobs[id]
			t.jobLogger(job.schedule).Infof("unschedule (no longer configured)")
			t.jobs.DelId(id)
		}
	}

	t.updateExposedSchedules(path)
}

func (t *T) updateExposedSchedules(path naming.Path) {
	table := t.schedules.Table(path)
	if table == nil {
		return
	}
	table = table.Merge(t.jobs.Table(path))
	schedule.TableData.Set(path, &table)
}

func (t *T) unscheduleRequireCollector() {
	for key, job := range t.jobs {
		if job.schedule.RequireCollector {
			job.Cancel()
			delete(t.jobs, key)
		}
	}
}

func (t *T) unschedule(path naming.Path) {
	t.reqSatisfied.UnsetPath(path)
	t.schedules.Del(path)
	t.jobs.DelPath(path)
}

func (t *T) publishUpdate() {
	t.status.UpdatedAt = time.Now()
	daemonsubsystem.DataScheduler.Set(t.localhost, t.status.DeepCopy())
	t.publisher.Pub(&msgbus.DaemonSchedulerUpdated{Node: t.localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", t.localhost})
}

func (t errMap) key(path naming.Path, s string) string {
	return fmt.Sprintf("%s:%s", path.String(), s)
}

func (t errMap) Get(path naming.Path, s string) (error, bool) {
	id := t.key(path, s)
	v, ok := t[id]
	return v, ok
}

func (t errMap) Set(path naming.Path, s string, err error) {
	id := t.key(path, s)
	t[id] = err
}

func (t errMap) Unset(path naming.Path, s string) {
	id := t.key(path, s)
	delete(t, id)
}

func (t errMap) UnsetPath(path naming.Path) {
	prefix := t.key(path, "")
	for k, _ := range t {
		if strings.HasPrefix(k, prefix) {
			t.Unset(path, k)
		}
	}
}

func (t timeMap) key(path naming.Path, s string) string {
	return fmt.Sprintf("%s:%s", path.String(), s)
}

func (t timeMap) Get(path naming.Path, s string) (time.Time, bool) {
	id := t.key(path, s)
	tm, ok := t[id]
	return tm, ok
}

func (t timeMap) Set(path naming.Path, s string, tm time.Time) {
	id := t.key(path, s)
	t[id] = tm
}

func (t timeMap) UnsetPath(path naming.Path) {
	prefix := t.key(path, "")
	for k, _ := range t {
		if strings.HasPrefix(k, prefix) {
			t.Unset(path, k)
		}
	}
}

func (t timeMap) Unset(path naming.Path, s string) {
	id := t.key(path, s)
	delete(t, id)
}

func (t timeMap) GetWithRID(path naming.Path, s string) (time.Time, bool) {
	return t.Get(path, s+".schedule")
}

func (t timeMap) SetWithRID(path naming.Path, s string, tm time.Time) {
	t.Set(path, s+".schedule", tm)
}
