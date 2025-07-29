package object

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/xsession"
)

type exitcoder interface {
	ExitCode() int
}

func (t *actor) FreshStatus(ctx context.Context) (instance.Status, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	return t.statusEval(ctx)
}

// MonitorStatus returns the service status dataset with monitored resources
// refreshed and non-monitore resources loaded from cache
func (t *actor) MonitorStatus(ctx context.Context) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	data, err = t.statusLoad()
	if err != nil {
		return t.FreshStatus(ctx)
	}
	return t.monitorStatusEval(ctx, data)
}

// Status returns the service status dataset
func (t *actor) Status(ctx context.Context) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	if t.statusDumpOutdated() {
		return t.statusEval(ctx)
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval(ctx)
}

func (t *actor) postActionStatusEval(ctx context.Context) {
	if _, err := t.statusEval(ctx); err != nil {
		t.log.Debugf("a status refresh is already in progress: %s", err)
	}
}

func (t *actor) monitorStatusEval(ctx context.Context, data instance.Status) (instance.Status, error) {
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedMonitorStatusEval(ctx, data)
}

func (t *actor) statusEval(ctx context.Context) (instance.Status, error) {
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedStatusEval(ctx)
}

func (t *actor) setLastStartedAt(data *instance.Status) error {
	stat, err := os.Stat(t.lastStartFile())
	switch {
	case errors.Is(err, os.ErrNotExist):
		data.LastStartedAt = time.Time{}
	case err != nil:
		return err
	default:
		data.LastStartedAt = stat.ModTime()
	}
	return nil
}

func (t *actor) lockedMonitorStatusEval(ctx context.Context, data instance.Status) (instance.Status, error) {
	t.setLastStartedAt(&data)
	data.UpdatedAt = time.Now()
	data.FrozenAt = t.Frozen()

	// reset fields that t.resourceStatusEval() will re-evaluate
	data.Avail = status.Undef
	data.Overall = status.Undef
	data.Provisioned = provisioned.Undef

	if err := t.resourceStatusEval(ctx, &data, true); err != nil {
		return data, fmt.Errorf("resource status eval: %w", err)
	}
	if len(data.Resources) == 0 {
		data.Avail = status.NotApplicable
		data.Overall = status.NotApplicable
		data.Optional = status.NotApplicable
	}
	var err error
	data.Running, err = mergedRunningInfoList(t)
	if err != nil {
		return data, fmt.Errorf("merge running resource: %w", err)
	}
	return data, t.statusDump(data)
}

func (t *actor) lockedStatusEval(ctx context.Context) (instance.Status, error) {
	var data instance.Status
	t.setLastStartedAt(&data)
	data.UpdatedAt = time.Now()
	data.FrozenAt = t.Frozen()
	if err := t.resourceStatusEval(ctx, &data, false); err != nil {
		return data, fmt.Errorf("resource status eval: %w", err)
	}
	if len(data.Resources) == 0 {
		data.Avail = status.NotApplicable
		data.Overall = status.NotApplicable
		data.Optional = status.NotApplicable
	}
	var err error
	data.Running, err = mergedRunningInfoList(t)
	if err != nil {
		return data, fmt.Errorf("merge running resource: %w", err)
	}
	err = t.statusDump(data)
	return data, err
}

func mergedRunningInfoList(t interface{}) (resource.RunningInfoList, error) {
	var errs error
	l := make(resource.RunningInfoList, 0)
	for _, r := range listResources(t) {
		if i, ok := r.(resource.Runninger); ok {
			runningInfoList, err := i.Running()
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("%s: %w", r.RID(), err))
			} else {
				l = append(l, runningInfoList...)
			}
		}
	}
	return l, errs
}

func (t *actor) isEncapNodeMatchingResource(r resource.Driver) (bool, error) {
	isEncapResource := r.IsEncap()
	isEncapNode, err := t.Config().IsInEncapNodes(hostname.Hostname())
	if err != nil {
		return false, err
	}
	if isEncapNode && isEncapResource {
		return true, nil
	}
	if !isEncapNode && !isEncapResource {
		return true, nil
	}
	return false, nil
}

func (t *actor) resourceStatusEval(ctx context.Context, data *instance.Status, monitoredOnly bool) error {
	if !monitoredOnly {
		data.Resources = make(instance.ResourceStatuses)
	}
	doResourceStatus := func(group driver.Group, resourceStatus resource.Status) {
		data.Overall.Add(resourceStatus.Status)
		if !resourceStatus.Optional {
			switch group {
			case driver.GroupSync:
			case driver.GroupTask:
			default:
				data.Avail.Add(resourceStatus.Status)
			}
		}
		data.Provisioned.Add(resourceStatus.Provisioned.State)
		for _, entry := range resourceStatus.Log {
			switch entry.Level {
			case resource.WarnLevel, resource.ErrorLevel:
				data.Overall.Add(status.Warn)
				break
			}
		}
	}
	var mu sync.Mutex
	sb := statusbus.FromContext(ctx)
	err := t.ResourceSets().Do(ctx, t, "", "status", func(ctx context.Context, r resource.Driver) error {
		var (
			resourceStatus      resource.Status
			encapInstanceStatus *instance.EncapStatus
			err                 error
		)

		if v, err := t.isEncapNodeMatchingResource(r); err != nil {
			return err
		} else if !v {
			return nil
		}

		if monitoredOnly && !r.IsMonitored() {
			resourceStatus = data.Resources[r.RID()]
			sb.Post(r.RID(), resourceStatus.Status, false)
		} else {
			resourceStatus = resource.GetStatus(ctx, r)
		}

		// If the resource is up but the provisioned flag is unset, set
		// the provisioned flag.
		if resourceStatus.Provisioned.State == provisioned.False {
			switch resourceStatus.Status {
			case status.Up, status.StandbyUp:
				resource.SetProvisioned(ctx, r)
				resourceStatus.Provisioned.State = provisioned.True
			}
		}

		// If the resource is a encap capable container, evaluate the encap instance
		if encapContainer, ok := r.(resource.Encaper); ok {
			if resourceStatus.Status.Is(status.Up, status.StandbyUp) {
				if encapInstanceStatus, err = t.resourceStatusEvalEncap(ctx, encapContainer, false); err != nil {
					log := resource.NewStatusLog(resourceStatus.Log...)
					log.Error("%s", err)
					resourceStatus.Log = log.Entries()
				}
			} else {
				encapInstanceStatus = &instance.EncapStatus{
					Status: instance.Status{
						Avail:   status.Down,
						Overall: status.Down,
					},
					Hostname: encapContainer.GetHostname(),
				}
			}
		}

		mu.Lock()
		data.Resources[r.RID()] = resourceStatus

		if encapInstanceStatus != nil {
			if data.Encap == nil {
				data.Encap = make(instance.EncapMap)
			}
			data.Encap[r.RID()] = *encapInstanceStatus
			for rid, encapResourceStatus := range encapInstanceStatus.Resources {
				resourceID, _ := resourceid.Parse(rid)
				doResourceStatus(resourceID.DriverGroup(), encapResourceStatus)
			}
		}
		doResourceStatus(r.ID().DriverGroup(), resourceStatus)

		mu.Unlock()
		return nil
	})
	mu.Lock()
	sb.Post("avail", data.Avail, false)
	sb.Post("overall", data.Overall, false)
	mu.Unlock()
	return err
}

func (t *actor) resourceStatusEvalEncap(ctx context.Context, encapContainer resource.Encaper, pushed bool) (*instance.EncapStatus, error) {
	var (
		encapInstanceStates *instance.States
		checksum            string
	)

	hostname := encapContainer.GetHostname()
	configFile := t.path.ConfigFile()

	if v, err := t.Config().IsInEncapNodes(hostname); err != nil {
		return nil, err
	} else if !v {
		return nil, nil
	}

	args := []string{encapContainer.GetOsvcRootPath(), t.path.String(), "instance", "status", "-r", "-o", "json"}
	envs := []string{
		"OSVC_SESSION_ID=" + xsession.ID.String(),
		env.ActionOrchestrationIDVar + "=" + os.Getenv(env.ActionOrchestrationIDVar),
		env.OriginSetenvArg(env.Origin()),
	}
	cmd, err := encapContainer.EncapCmd(ctx, args, envs)
	if err != nil {
		return nil, err
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(exitcoder); ok {
			if exitErr.ExitCode() == 2 {
				if pushed {
					return nil, fmt.Errorf("no encap instance config: already pushed")
				}
				t.log.Debugf("%s: no encap instance config: push the config", t.path)
				if err := encapContainer.EncapCp(ctx, configFile, configFile); err != nil {
					return nil, err
				}
				return t.resourceStatusEvalEncap(ctx, encapContainer, true)
			}
		}
		return nil, fmt.Errorf("encap instance status: %w: %s", err, strings.TrimSpace(string(b)))
	}
	var encapInstanceStatesList instance.StatesList
	if err := json.Unmarshal(b, &encapInstanceStatesList); err != nil {
		return nil, err
	}
	if len(encapInstanceStatesList) == 0 {
		if pushed {
			return nil, fmt.Errorf("no encap instance status: already pushed")
		}
		t.log.Debugf("%s: no encap instance status: push the config", t.path)
		if err := encapContainer.EncapCp(ctx, configFile, configFile); err != nil {
			return nil, err
		}
		return t.resourceStatusEvalEncap(ctx, encapContainer, true)
	}
	for _, e := range encapInstanceStatesList {
		if hostname == e.Node.Name {
			encapInstanceStates = &e
			break
		}
	}
	if encapInstanceStates == nil {
		return nil, fmt.Errorf("no instance states found for node %s", hostname)
	}
	if checksum == "" {
		if b, err := file.MD5(configFile); err != nil {
			return nil, fmt.Errorf("config file %s not found for md5sum", configFile)
		} else {
			checksum = fmt.Sprintf("%x", b)
		}
	}
	if encapInstanceStates.Config.Checksum != checksum {
		if pushed {
			return nil, fmt.Errorf("encap instance config checksum (%s) is different than host's (%s): already pushed", encapInstanceStates.Config.Checksum, checksum)
		}
		t.log.Debugf("%s: encap instance config checksum (%s) is different than host's (%s): push the config", t.path, encapInstanceStates.Config.Checksum, checksum)
		if err := encapContainer.EncapCp(ctx, configFile, configFile); err != nil {
			return nil, err
		}
		return t.resourceStatusEvalEncap(ctx, encapContainer, true)
	}

	encapInstanceStatus := instance.EncapStatus{
		Hostname: hostname,
		Status:   encapInstanceStates.Status,
	}
	return &encapInstanceStatus, nil
}
