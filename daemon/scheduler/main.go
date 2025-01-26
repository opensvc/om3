package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/runfiles"
)

type (
	T struct {
		ctx       context.Context
		cancel    context.CancelFunc
		log       *plog.Logger
		localhost string
		databus   *daemondata.T
		pub       pubsub.PublishBuilder

		events      chan any
		jobs        Jobs
		enabled     bool
		provisioned map[naming.Path]bool
		schedules   Schedules

		wg sync.WaitGroup

		subQS pubsub.QueueSizer

		status daemonsubsystem.Scheduler

		maxRunning int
	}

	Schedules map[string]map[string]schedule.Entry

	Jobs map[string]Job
	Job  struct {
		CreatedAt time.Time
		LastRunAt time.Time
		schedule  schedule.Entry
		cancel    []func()
	}
	eventJobAlarm struct {
		schedule schedule.Entry
	}
	eventJobDone struct {
		schedule schedule.Entry
		end      time.Time
	}
	eventJobRun struct {
		schedule schedule.Entry
		begin    time.Time
	}
)

var (
	needProvisionedInstanceMap = map[string]any{
		"sync_update":      nil,
		"compliance_auto":  nil,
		"resource_monitor": nil,
		"run":              nil,
	}

	incompatibleNodeMonitorStatus = map[node.MonitorState]any{
		node.MonitorStateInit:             nil,
		node.MonitorStateMaintenance:      nil,
		node.MonitorStateRejoin:           nil,
		node.MonitorStateShutdownProgress: nil,
		node.MonitorStateUpgrade:          nil,
	}
)

func needProvisionedInstance(action string) bool {
	_, ok := needProvisionedInstanceMap[action]
	return ok
}

func (t Schedules) DelByPath(path naming.Path) {
	delete(t, path.String())
}

func (t Schedules) Add(path naming.Path, e schedule.Entry) {
	pathStr := path.String()
	if _, ok := t[pathStr]; !ok {
		t[pathStr] = make(map[string]schedule.Entry)
	}
	t[pathStr][e.Key] = e
}

func (t Schedules) Get(path naming.Path, k string) (schedule.Entry, bool) {
	if m, ok := t[path.String()]; !ok {
		return schedule.Entry{}, false
	} else if e, ok := m[k]; !ok {
		return schedule.Entry{}, false
	} else {
		return e, true
	}
}

func New(subQS pubsub.QueueSizer, opts ...funcopt.O) *T {
	t := &T{
		log:         plog.NewDefaultLogger().Attr("pkg", "daemon/scheduler").WithPrefix("daemon: scheduler: "),
		localhost:   hostname.Hostname(),
		events:      make(chan any),
		jobs:        make(Jobs),
		schedules:   make(Schedules),
		provisioned: make(map[naming.Path]bool),
		subQS:       subQS,

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
			schedule:  e,
		}
	}
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

func (t Jobs) Del(e schedule.Entry) {
	jobId := newJobId(e)
	job, ok := t[jobId]
	if !ok {
		return
	}
	job.Cancel()
	delete(t, jobId)
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

func (t *T) createJob(e schedule.Entry) {
	if !t.enabled {
		return
	}

	logger := t.jobLogger(e)
	now := time.Now() // keep before GetNext call
	next, _, err := e.GetNext()
	if err != nil {
		logger.Attr("definition", e.Schedule).Warnf("failed to find a next date: %s", err)
		t.jobs.Del(e)
		return
	}
	if next.IsZero() {
		t.jobs.Del(e)
		return
	}
	if next.Before(now) {
		logger.Warnf("last %s, next %s is in the past", e.LastRunAt, next)
		t.jobs.Del(e)
		return
	}
	e.NextRunAt = next
	delay := next.Sub(now)
	if delay >= time.Second {
		logger.Infof("next at %s (in %s)", next, delay)
	}
	t.jobs.Add(e, delay, t.events)
	return
}

func (t *T) jobLogger(e schedule.Entry) *plog.Logger {
	logger := naming.LogWithPath(t.log, e.Path).Attr("action", e.Action).Attr("key", e.Key)
	var obj string
	if e.Path.IsZero() {
		obj = "node"
	} else {
		obj = e.Path.String()
	}
	var prefix string
	if rid := e.RID(); rid != "DEFAULT" {
		prefix = fmt.Sprintf("%s%s: %s: %s: ", t.log.Prefix(), obj, rid, e.Action)
	} else {
		prefix = fmt.Sprintf("%s%s: %s: ", t.log.Prefix(), obj, e.Action)
	}
	return logger.WithPrefix(prefix)
}

func (t *T) onJobAlarm(c eventJobAlarm) {
	logger := t.jobLogger(c.schedule)
	e, ok := t.schedules.Get(c.schedule.Path, c.schedule.Key)
	if !ok {
		logger.Infof("aborted, schedule is gone")
		return
	}
	// plan the next run before exec, so another exec can be done
	// even if another is running
	e.LastRunAt = c.schedule.LastRunAt
	e.NextRunAt = c.schedule.NextRunAt
	t.recreateJobFrom(e, c.schedule.NextRunAt)

	go func() {
		if n, err := t.runningCount(e); err != nil {
			logger.Warnf("%s", err)
		} else if n >= e.MaxParallel {
			logger.Infof("aborted, %d/%d jobs already running", n, e.MaxParallel)
		} else if e.RequireCollector && !collector.Alive.Load() {
			logger.Debugf("the collector is not alive")
		} else {
			t.events <- eventJobRun{
				schedule: e,
				begin:    c.schedule.NextRunAt,
			}
			if err := t.action(e); err != nil {
				logger.Errorf("on exec %s: %s", e.Key, err)
			} else {
				// remember last success, for users benefit
				if err := e.SetLastSuccess(c.schedule.NextRunAt); err != nil {
					logger.Errorf("on update last success %s: %s", e.Key, err)
				}
			}

			// remember last run, to not run the job too soon after a daemon restart
			if err := e.SetLastRun(c.schedule.NextRunAt); err != nil {
				logger.Errorf("on update last run %s: %s", e.Key, err)
			}

			t.events <- eventJobDone{
				schedule: e,
				end:      time.Now(),
			}
		}
	}()
}

func (t *T) runningCount(e schedule.Entry) (int, error) {
	if e.RunDir == "" {
		return -1, nil
	}
	logger := t.jobLogger(e)
	dir := runfiles.Dir{
		Path: e.RunDir,
		Log:  logger,
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
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, labelLocalhost)
	sub.AddFilter(&msgbus.ObjectStatusDeleted{}, labelLocalhost)
	sub.AddFilter(&msgbus.ObjectStatusUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeMonitorUpdated{}, labelLocalhost)
	sub.Start()
	return sub
}

func (t *T) loop() {
	t.log.Debugf("loop started")
	t.databus = daemondata.FromContext(t.ctx)
	t.pub = pubsub.PubFromContext(t.ctx)
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
			case *msgbus.InstanceConfigUpdated:
				t.onInstConfigUpdated(c)
			case *msgbus.InstanceStatusDeleted:
				t.onInstStatusDeleted(c)
			case *msgbus.NodeMonitorUpdated:
				t.onNodeMonitorUpdated(c)
			case *msgbus.NodeConfigUpdated:
				t.onNodeConfigUpdated(c)
			case *msgbus.ObjectStatusUpdated:
				t.onMonObjectStatusUpdated(c)
			}
		case ev := <-t.events:
			switch c := ev.(type) {
			case eventJobAlarm:
				t.onJobAlarm(c)
			case eventJobDone:
				t.onJobDone(c)
			case eventJobRun:
				t.onJobRun(c)
			default:
				t.log.Errorf("received an unsupported event: %#v", c)
			}
		case <-t.ctx.Done():
			t.jobs.Purge()
			return
		}
	}
}

func (t *T) onJobRun(c eventJobRun) {
	jobId := newJobId(c.schedule)
	job, ok := t.jobs[jobId]
	if !ok {
		return
	}
	job.LastRunAt = c.begin
	t.jobs[jobId] = job
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
}

func (t *T) onInstStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	t.loggerWithPath(c.Path).Infof("%s: unschedule all jobs (instance deleted)", c.Path)
	t.unschedule(c.Path)
}

func (t *T) onMonObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	isProvisioned := c.Value.Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable)
	wasProvisioned, ok := t.provisioned[c.Path]
	t.provisioned[c.Path] = isProvisioned
	if !ok || (isProvisioned != wasProvisioned) {
		t.scheduleObject(c.Path)
	}
}

func (t *T) loggerWithPath(path naming.Path) *plog.Logger {
	return naming.LogWithPath(t.log, path)
}

func (t *T) onInstConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	switch {
	case t.enabled:
		t.loggerWithPath(c.Path).Infof("%s: update schedules", c.Path)
		t.unschedule(c.Path)
		t.scheduleObject(c.Path)
	}
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
		t.log.Infof("node: update schedules")
		t.unschedule(naming.Path{})
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
	for _, p := range object.StatusData.GetPaths() {
		t.scheduleObject(p)
	}
	t.scheduleNode()
}

func (t *T) reschedule(p naming.Path, isProvisioned bool) {
}

func (t *T) schedule(p naming.Path) {
	if !t.enabled {
		return
	}
	if p.IsZero() {
		t.scheduleNode()
	} else {
		t.scheduleObject(p)
	}
}

func (t *T) scheduleNode() {
	o, err := object.NewNode()
	if err != nil {
		t.log.Errorf("node: %s", err)
		return
	}

	table := o.PrintSchedule()
	defer schedule.TableData.Set(naming.Path{}, &table)

	for _, e := range table {
		t.schedules.Add(naming.Path{}, e)
		t.createJob(e)
	}
	table = table.Merge(t.jobs.Table(naming.Path{}))
}

func (t *T) scheduleObject(path naming.Path) {
	log := t.loggerWithPath(path).WithPrefix(t.log.Prefix() + path.String() + ": ")
	i, err := object.New(path, object.WithVolatile(true))
	if err != nil {
		log.Errorf("%s", err)
		return
	}
	o, ok := i.(object.Actor)
	if !ok {
		// only actor objects have scheduled actions
		return
	}

	table := o.PrintSchedule()
	defer schedule.TableData.Set(path, &table)

	isProvisioned, ok := t.provisioned[path]
	if !ok {
		log.Infof("provisioned state has not been discovered yet")
		return
	}

	for _, e := range table {
		if !isProvisioned && needProvisionedInstance(e.Action) {
			if t.jobs.Has(e) {
				log.Infof("unschedule %s %s (instance no longer provisionned)", e.RID(), e.Action)
				t.jobs.Del(e)
			} else {
				log.Infof("skip schedule %s %s: instance not provisioned", e.RID(), e.Action)
			}
			continue
		}
		t.schedules.Add(path, e)
		t.createJob(e)
	}
	table = table.Merge(t.jobs.Table(path))
}

func (t *T) unschedule(p naming.Path) {
	t.schedules.DelByPath(p)
	t.jobs.DelPath(p)
}

func (t *T) publishUpdate() {
	t.status.UpdatedAt = time.Now()
	localhost := hostname.Hostname()
	daemonsubsystem.DataScheduler.Set(localhost, t.status.DeepCopy())
	t.pub.Pub(&msgbus.DaemonSchedulerUpdated{Node: localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", localhost})
}
