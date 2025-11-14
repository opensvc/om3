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

	"github.com/labstack/gommon/log"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kwoption"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/resourcereqs"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/core/status"
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
		publisher pubsub.Publisher

		events              chan any
		jobs                Jobs
		enabled             bool
		provisioned         map[naming.Path]bool
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
		reqSatisfied boolMap
	}

	Schedules map[naming.Path]map[string]schedule.Entry

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

	timeMap map[string]time.Time
	boolMap map[string]bool
)

var (
	incompatibleNodeMonitorStatus = map[node.MonitorState]any{
		node.MonitorStateInit:             nil,
		node.MonitorStateMaintenance:      nil,
		node.MonitorStateRejoin:           nil,
		node.MonitorStateShutdownProgress: nil,
		node.MonitorStateUpgrade:          nil,
	}
)

func (t Schedules) DelByPath(path naming.Path) {
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
		provisioned:       make(map[naming.Path]bool),
		subQS:             subQS,
		lastRunOnAllPeers: make(timeMap),
		reqSatisfied:      make(boolMap),

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
	if e.RequireCollector && !t.isCollectorJoinable {
		return
	}
	if e.Require != "" {
		if isSatisfied, ok := t.reqSatisfied.Get(e.Path, e.Key); !ok || !isSatisfied {
			return
		}
	}

	logger := t.jobLogger(e)
	if e.LastRunAt.IsZero() {
		// after daemon start: initialize the schedule's LastRunAt from LastRunFile
		e.LastRunAt = e.GetLastRun()
	}
	if lastRunOnAllPeers, ok := t.lastRunOnAllPeers.Get(e.Path, e.Key); ok && e.LastRunAt.Before(lastRunOnAllPeers) {
		logger.Infof("adjust schedule entry last run time: %s => %s", e.LastRunAt, lastRunOnAllPeers)
		e.LastRunAt = lastRunOnAllPeers
	}

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
		logger.Warnf("next %s is in the past", next)
		t.jobs.Del(e)
		return
	}
	e.NextRunAt = next
	delay := next.Sub(now)
	if e.LastRunAt.IsZero() || delay >= time.Second {
		logger.Debugf("next at %s (in %s)", next, delay)
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

func (t *T) isProvisioned(path naming.Path) bool {
	isProvisioned, hasProvisioned := t.provisioned[path]
	return hasProvisioned && isProvisioned
}

func (t *T) onJobAlarm(c eventJobAlarm) {
	logger := t.jobLogger(c.schedule)
	e, ok := t.schedules.Get(c.schedule.Path, c.schedule.Key)
	if !ok {
		logger.Infof("aborted, schedule is gone")
		return
	}
	if e.RequireCollector && !t.isCollectorJoinable {
		logger.Infof("aborted, the collector is not joinable")
		return
	}
	if !e.Path.IsZero() {
		if e.RequireProvisioned && !t.isProvisioned(e.Path) {
			logger.Infof("%s: aborted, the object is no longer provisioned", e.RID())
			return
		}
		if isSatisfied, ok := t.reqSatisfied.Get(e.Path, e.Key); ok {
			if !isSatisfied {
				log.Infof("%s: aborted, requirements no longer met", e.RID())
				return
			}
		} else if e.Require != "" {
			log.Infof("%s: aborted, requirements not yet evaluated", e.RID())
			return
		}
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
		logger.Infof("aborted, %d/%d jobs already running", n, e.MaxParallel)
		return
	}
	go func() {
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
	sub.AddFilter(&msgbus.DaemonCollectorUpdated{}, labelLocalhost)
	sub.Start()
	return sub
}

func (t *T) loop() {
	t.log.Debugf("loop started")
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
			case *msgbus.InstanceConfigUpdated:
				t.onInstanceConfigUpdated(c)
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
	t.lastRunOnAllPeers.Set(c.schedule.Path, c.schedule.Key, c.schedule.NextRunAt)
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
	t.updateExposedSchedules(prev.Path)
}

func (t *T) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	t.loggerWithPath(c.Path).Infof("%s: unschedule all jobs (instance deleted)", c.Path)
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

	log := t.loggerWithPath(c.Path)
	changed := false

	for _, e := range schedules {
		if e.Require == "" {
			continue
		}
		reqs := resourcereqs.New(e.Require)
		for requiredRID, requiredStatusList := range reqs.Requirements() {
			err := checkReq(requiredRID, requiredStatusList)
			currentlySatisfied, ok := t.reqSatisfied.Get(c.Path, e.Key)
			if err != nil {
				if !ok {
					t.reqSatisfied.Set(c.Path, e.Key, false)
					log.Infof("%s: %s: requirement unsatisfied: %s", c.Path, e.RID(), err)
					changed = true
				} else if currentlySatisfied {
					t.reqSatisfied.Set(c.Path, e.Key, false)
					log.Infof("%s: %s: requirement no longer satisfied: %s", c.Path, e.RID(), err)
					changed = true
				}
			} else {
				if !ok {
					t.reqSatisfied.Set(c.Path, e.Key, true)
					log.Infof("%s: %s: requirement satisfied", c.Path, e.RID())
					changed = true
				} else if !currentlySatisfied {
					t.reqSatisfied.Set(c.Path, e.Key, true)
					log.Infof("%s: %s: requirement now satisfied", c.Path, e.RID())
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
	log := t.loggerWithPath(c.Path)
	for rid, r := range c.Value.Resources {
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
				log.Infof("%s: %s: initialize last run at %s on %s", c.Path, rid, tm, nodename)
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
			log.Warnf("%s: %s: write last run on file: %s", c.Path, rid, err)
		}

		cachedLastRunAtOnPeer, ok := t.lastRunOnAllPeers.GetWithRID(c.Path, rid)

		if !ok || lastRunAtOnPeer.After(cachedLastRunAtOnPeer) {
			log.Debugf("%s: %s: last run on peer %s at %s", c.Path, rid, c.Node, lastRunAtOnPeer)
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
			for key, job := range t.jobs {
				if job.schedule.RequireCollector {
					job.Cancel()
					delete(t.jobs, key)
				}
			}
			t.scheduleAll()
		}
	}
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.lastRunOnAllPeers.UnsetPath(c.Path)
	t.reqSatisfied.UnsetPath(c.Path)
}

func (t *T) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	if c.Value.ActorStatus == nil {
		return
	}
	changed := false
	if srcEv, ok := c.SrcEv.(*msgbus.InstanceStatusUpdated); ok {
		if t.onInstanceStatusUpdated(srcEv) {
			changed = true
		}
	}
	if c.Value.Provisioned == provisioned.Undef {
		delete(t.provisioned, c.Path)
		return
	}
	isProvisioned := c.Value.Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable)
	wasProvisioned, ok := t.provisioned[c.Path]
	t.provisioned[c.Path] = isProvisioned
	if !ok || (isProvisioned != wasProvisioned) || changed {
		t.scheduleObject(c.Path)
	}
}

func (t *T) loggerWithPath(path naming.Path) *plog.Logger {
	return naming.LogWithPath(t.log, path)
}

func (t *T) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if c.Value.ActorConfig == nil {
		return
	}
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
	for p, _ := range instance.StatusData.GetByNode(t.localhost) {
		t.scheduleObject(p)
	}
	t.scheduleNode()
}

func (t *T) schedule(p naming.Path) {
	if p.IsZero() {
		t.scheduleNode()
	} else {
		t.scheduleObject(p)
	}
}

func (t *T) scheduleNode() {
	if !t.enabled {
		return
	}
	o, err := object.NewNode()
	if err != nil {
		t.log.Errorf("node: %s", err)
		return
	}

	path := naming.Path{}
	table := o.Schedules()
	defer t.updateExposedSchedules(path)

	for _, e := range table {
		t.schedules.Add(path, e)
		t.createJob(e)
	}
}

func (t *T) scheduleObject(path naming.Path) {
	if !t.enabled {
		return
	}
	log := t.loggerWithPath(path).WithPrefix(t.log.Prefix() + path.String() + ": ")

	instanceConfig := instance.ConfigData.GetByPathAndNode(path, t.localhost)
	if instanceConfig == nil || instanceConfig.ActorConfig == nil || instanceConfig.Schedules == nil || len(instanceConfig.Schedules) == 0 {
		// only actor objects have scheduled actions
		return
	}

	defer t.updateExposedSchedules(path)

	isProvisioned, hasProvisioned := t.provisioned[path]

	for _, scheduleConfig := range instanceConfig.Schedules {
		e := schedule.Entry{
			Node:   t.localhost,
			Path:   path,
			Config: scheduleConfig,
		}
		t.schedules.Add(path, e)
		if e.Schedule == "" || e.Schedule == "@0" {
			if t.jobs.Has(e) {
				log.Infof("%s: unschedule %s (schedule is now @0)", e.RID(), e.Action)
				t.jobs.Del(e)
			}
			continue
		}
		if e.RequireProvisioned {
			if !hasProvisioned {
				log.Infof("%s: skip schedule %s (instance provisioned state is still unknown)", e.RID(), e.Action)
				t.jobs.Del(e)
				continue
			}
			if !isProvisioned {
				if t.jobs.Has(e) {
					log.Infof("%s: unschedule %s (instance no longer provisionned)", e.RID(), e.Action)
					t.jobs.Del(e)
				} else {
					log.Infof("%s: skip schedule %s (instance not provisioned)", e.RID(), e.Action)
				}
				continue
			}
		}
		if isSatisfied, ok := t.reqSatisfied.Get(path, e.Key); ok {
			if !isSatisfied {
				if t.jobs.Has(e) {
					log.Infof("%s: unschedule %s (requirements no longer met)", e.RID(), e.Action)
					t.jobs.Del(e)
				} else {
					log.Infof("%s: skip schedule %s (requirements not met)", e.RID(), e.Action)
				}
				continue
			}

		} else if e.Require != "" {
			if t.jobs.Has(e) {
				log.Infof("%s: unschedule %s (requirements not yet evaluated)", e.RID(), e.Action)
				t.jobs.Del(e)
			} else {
				log.Infof("%s: skip schedule %s (requirements not yet evaluated)", e.RID(), e.Action)
			}
			continue
		}
		t.schedules.Add(path, e)
		t.createJob(e)
	}
}

func (t *T) updateExposedSchedules(path naming.Path) {
	table := t.schedules.Table(path)
	if table == nil {
		return
	}
	table = table.Merge(t.jobs.Table(path))
	schedule.TableData.Set(path, &table)
}

func (t *T) unschedule(path naming.Path) {
	t.schedules.DelByPath(path)
	t.jobs.DelPath(path)
}

func (t *T) publishUpdate() {
	t.status.UpdatedAt = time.Now()
	daemonsubsystem.DataScheduler.Set(t.localhost, t.status.DeepCopy())
	t.publisher.Pub(&msgbus.DaemonSchedulerUpdated{Node: t.localhost, Value: *t.status.DeepCopy()}, pubsub.Label{"node", t.localhost})
}

func (t boolMap) key(path naming.Path, s string) string {
	return fmt.Sprintf("%s:%s", path.String(), s)
}

func (t boolMap) Get(path naming.Path, s string) (bool, bool) {
	id := t.key(path, s)
	v, ok := t[id]
	return v, ok
}

func (t boolMap) Set(path naming.Path, s string, v bool) {
	id := t.key(path, s)
	t[id] = v
}

func (t boolMap) Unset(path naming.Path, s string) {
	id := t.key(path, s)
	delete(t, id)
}

func (t boolMap) UnsetPath(path naming.Path) {
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
