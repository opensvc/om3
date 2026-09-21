package object

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/xsession"
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
		t.log.Tracef("a status refresh is already in progress: %s", err)
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
	// The resources are configured once here, and not again before each of
	// them is evaluated.
	//
	// An action configures a resource again right before it acts, because a
	// keyword of one resource can reference what another resource exposes
	// once started, and that value only exists after the referenced resource
	// has run. Evaluating the status changes no state, so nothing a keyword
	// reads can move between here and the end of the pass, and configuring
	// each resource a second time on its turn would evaluate every keyword of
	// the object twice for one status.
	//
	// This also rebuilds the resources, which is what keeps the status
	// refresh that closes an action from reading what the action saw: the
	// drivers it evaluates are not the ones the action just used, so whatever
	// they had cached while changing the state is gone. That separation is a
	// policy, not an optimisation, and it is this call that holds it.
	t.ConfigureResources()

	if !monitoredOnly {
		data.Resources = make(instance.ResourceStatuses)
	}
	doResourceStatus := func(group driver.Group, resourceStatus resource.Status) {
		data.Overall.Add(resourceStatus.Status)
		if !resourceStatus.IsOptional {
			switch group {
			case driver.GroupSync:
			case driver.GroupTask:
			default:
				data.Avail.Add(resourceStatus.Status)
			}
		}
		data.Provisioned.Add(resourceStatus.IsProvisioned.State)
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
		)

		if v, err := t.isEncapNodeMatchingResource(r); err != nil {
			return err
		} else if !v {
			return nil
		}

		if err := r.GetConfigurationError(); err != nil {
			log := resource.NewStatusLog()
			log.Error("%s", err)
			resourceStatus.Log = log.Entries()
			data.Resources[r.RID()] = resourceStatus
			return nil
		}

		if monitoredOnly && !r.IsMonitored() {
			resourceStatus = data.Resources[r.RID()]
			sb.Post(r.RID(), resourceStatus.Status, false)
		} else {
			resourceStatus = resource.GetStatus(ctx, r)
		}

		if msg := t.resizeShortfall(ctx, r); msg != "" {
			log := resource.NewStatusLog(resourceStatus.Log...)
			log.Warn("%s", msg)
			resourceStatus.Log = log.Entries()
		}

		// If the resource is up but the provisioned flag is unset, set
		// the provisioned flag.
		if resourceStatus.IsProvisioned.State == provisioned.False {
			switch resourceStatus.Status {
			case status.Up, status.StandbyUp:
				resource.SetProvisioned(ctx, r)
				resourceStatus.IsProvisioned.State = provisioned.True
			}
		}

		// If the resource is a encap capable container, evaluate the encap instance
		if encapNodes, err := t.EncapNodes(); err != nil {
			return err
		} else if len(encapNodes) > 0 {
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
	// No resource contributed to the aggregated status when the object has no
	// resources at all, or when all its resources are excluded from the
	// aggregation, like the task and sync ones for avail. Report not
	// applicable instead of leaving the zero value undef.
	if data.Avail == status.Undef {
		data.Avail = status.NotApplicable
	}
	if data.Overall == status.Undef {
		data.Overall = status.NotApplicable
	}
	sb.Post("avail", data.Avail, false)
	sb.Post("overall", data.Overall, false)
	mu.Unlock()
	return err
}

func (t *actor) installEncapConfig(ctx context.Context, encapContainer resource.Encaper, configFile string) error {
	// Prepare a io.Reader to serve the config
	r, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer func() { r.Close() }()

	pipeReader, pipeWriter := io.Pipe()

	// Execute `om <path> create --config=- --restore --wait` in the encap container with the config piped in
	args := []string{encapContainer.GetOsvcRootPath(), t.path.String(), "create", "--config=-", "--restore", "--wait"}
	envs := []string{
		xsession.SessionID().Var(),
		env.Origin().Var(),
	}
	if v := xsession.OrchestrationID().Var(); v != "" {
		envs = append(envs, v)
	}
	cmd, err := encapContainer.EncapCmd(ctx, args, envs, pipeReader)
	if err != nil {
		pipeReader.Close()
		pipeWriter.Close()
		return err
	}
	go func() {
		defer pipeWriter.Close() // signal EOF to the command.
		io.Copy(pipeWriter, r)
	}()
	return cmd.Run()
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
		xsession.SessionID().Var(),
		env.Origin().Var(),
	}
	if v := xsession.OrchestrationID().Var(); v != "" {
		envs = append(envs, v)
	}
	cmd, err := encapContainer.EncapCmd(ctx, args, envs, nil)
	if err != nil {
		return nil, err
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(exitcoder); ok {
			if exitErr.ExitCode() == xerrors.ExitCodeObjectNotFound {
				if pushed {
					return nil, fmt.Errorf("no encap instance config: already pushed")
				}
				t.log.Tracef("%s: no encap instance config: push the config", t.path)
				if err := t.installEncapConfig(ctx, encapContainer, configFile); err != nil {
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
		t.log.Tracef("%s: no encap instance status: push the config", t.path)
		if err := t.installEncapConfig(ctx, encapContainer, configFile); err != nil {
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
		t.log.Tracef("%s: encap instance config checksum (%s) is different than host's (%s): push the config", t.path, encapInstanceStates.Config.Checksum, checksum)
		if err := t.installEncapConfig(ctx, encapContainer, configFile); err != nil {
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

// resizeShortfall says a resource holds less than the size it is configured
// to hold, and nothing when it holds it or when the question does not apply.
//
// The configured size is the target every node converges to, so a resource
// short of it is a resize that stopped part way, which no other reading says:
// a link whose own stage succeeded recorded the size it reached, so its
// keyword and its size agree, and only the target it was growing towards
// disagrees.
//
// A size that cannot be read says nothing. A stopped resource has no size to
// compare, and failing to read one is not a reason to warn about it.
func (t *actor) resizeShortfall(ctx context.Context, r resource.Driver) string {
	sizer, ok := r.(resource.Sizer)
	if !ok {
		return ""
	}
	current, err := sizer.CurrentSize(ctx)
	if err != nil || current <= 0 {
		return ""
	}

	// The keyword is declared by the driver whether or not the configuration
	// sets it, so what is set is read rather than what it converts to: an
	// unset size converts to zero, not to nothing.
	k := key.T{Section: r.RID(), Option: "size"}
	if t.config.Get(k) == "" {
		// The size of the object is not this resource's target. It is what
		// the volume was claimed for from its pool, which the pool hands to
		// the bottom of the chain, and every link above keeps a cut of it
		// for its own metadata. Comparing a link to it warns for ever about
		// a shortfall the layout has by construction.
		return t.resizeSpanShortfall(ctx, r, current)
	}
	configured := t.config.GetSize(k)
	if configured == nil || *configured <= 0 {
		return ""
	}
	if current >= *configured {
		return ""
	}
	held, target := resizeSizePair(current, *configured)

	// Whether a resize would make up the difference is the resource's to say,
	// and it says it by refusing to plan one: a raid0 array holds what its
	// members give it and grows by taking another member, so an array
	// configured for more than its members hold is not a resize that stopped
	// part way. It is a size it will never hold, which is worth saying once
	// rather than reporting for ever as unfinished work.
	resizer, ok := r.(resource.Resizer)
	if !ok {
		return fmt.Sprintf("holds %s of the %s it is configured to hold, and cannot be resized", held, target)
	}
	if _, err := resizer.ResizePlan(ctx, *configured); err != nil {
		return fmt.Sprintf("holds %s of the %s it is configured to hold, and cannot grow to it: %s", held, target, err)
	}
	return fmt.Sprintf("holds %s of the %s it is configured to hold, so a resize has not finished", held, target)
}

// resizeSpanShortfall says a resource has not taken all of what is under it.
//
// A resource with no size keyword of its own has no target written anywhere,
// so what says it is behind is the device under it holding more than it asks
// of it. A chain grows from the bottom up, and one that stopped part way is
// exactly that: space made below that nothing above has taken.
//
// Only a resource grown onto what is under it can be read this way. A logical
// volume takes a part of its volume group and leaves the rest, so holding
// less than what is under it says nothing about it.
//
// What the resource asks of the device below is its own plan for the size it
// already holds. That is what makes the reading exact where a subtraction
// would not be: the cut a link keeps for itself is the link's to compute, and
// a link that over-asks, as the drbd metadata does by design, reads as having
// taken everything rather than as being short of it.
func (t *actor) resizeSpanShortfall(ctx context.Context, r resource.Driver, current int64) string {
	spanner, ok := r.(resource.ResizeSpansBelow)
	if !ok || !spanner.ResizeSpansBelow() {
		return ""
	}
	resizer, ok := r.(resource.Resizer)
	if !ok {
		return ""
	}
	subDeviceser, ok := r.(resource.SubDeviceser)
	if !ok {
		return ""
	}
	devs := subDeviceser.SubDevices(ctx)
	if len(devs) != 1 {
		// Which of several devices a resource has not taken is not something
		// this can decide, and a resize refuses such a chain anyway.
		return ""
	}
	below, err := devs[0].Size()
	if err != nil || below <= 0 {
		return ""
	}
	need, err := resizer.ResizePlan(ctx, current)
	if err != nil || need >= below {
		return ""
	}
	held, has := resizeSizePair(current, below)
	return fmt.Sprintf("holds %s while the %s under it holds %s, so a resize has not finished", held, devs[0].Path(), has)
}

// resizeSizePair renders two sizes so that the difference between them shows.
//
// The compact rendering is the readable one and is kept while the two differ
// under it. A resource short by less than the rendering resolves prints as
// holding what it is configured to hold, which reads as a warning about
// nothing: the drbd of a volume keeps its metadata out of what it hands up,
// and is short by that much for ever.
func resizeSizePair(current, configured int64) (string, string) {
	held := sizeconv.BSizeCompact(float64(current))
	target := sizeconv.BSizeCompact(float64(configured))
	if held != target {
		return held, target
	}
	return fmt.Sprintf("%d bytes", current), fmt.Sprintf("%d bytes", configured)
}
