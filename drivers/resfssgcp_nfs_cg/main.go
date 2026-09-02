package resfssgcp_nfs_cg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/sgcphelper"
	"github.com/opensvc/om3/v3/util/ageingcache"
	"github.com/opensvc/om3/v3/util/httpclientcache"
	"github.com/opensvc/om3/v3/util/sgcp"
)

const (
	waitMsgInterval = 10 * time.Second

	cacheKeyGetCgInfo = "getSGCPCGInfo"
)

var (
	retryWaitDelay = 2 * time.Second

	ErrAlreadyResumed   = errors.New("already resumed")
	ErrResumeInProgress = errors.New("resume in progress")
	ErrPrecondition     = errors.New("precondition error")
)

type (
	AZStatus struct {
		AvailabilityZone string `json:"availabilityZone"`
		Status           string `json:"status"`
	}
	GeoRedundancyInfo struct {
		Region                  string     `json:"region"`
		TargetAvailabilityZones []AZStatus `json:"targetAvailabilityZones"`
	}
	ReplicationInfo struct {
		ReplicationMode         string     `json:"replicationMode"`
		TargetAvailabilityZones []AZStatus `json:"targetAvailabilityZones"`
	}
	GeoTargetDetail struct {
		Region string
		AZ     string
		Status string
	}
	RepTargetDetail struct {
		Mode   string
		AZ     string
		Status string
	}
	CgInfo struct {
		UUID             string            `json:"uuid"`
		Name             string            `json:"name"`
		AvailabilityZone string            `json:"availabilityZone"`
		Status           string            `json:"status"`
		GeoRedundancy    GeoRedundancyInfo `json:"georedundancy"`
		Replication      ReplicationInfo   `json:"replication"`
	}
)

func (cg *CgInfo) String() string {
	if cg == nil {
		return "NfsCg <nil>"
	}
	return fmt.Sprintf("NfsCg uuid:%s, name:%s, status:%s az:%s geo_redundancy:%+v replication:%+v",
		cg.UUID, cg.Name, cg.Status, cg.AvailabilityZone, cg.GeoRedundancy, cg.Replication)
}

func (cg *CgInfo) GeoRedundancies() []GeoTargetDetail {
	region := cg.GeoRedundancy.Region
	if region == "" {
		region = "undef"
	}
	details := make([]GeoTargetDetail, 0, len(cg.GeoRedundancy.TargetAvailabilityZones))
	for _, target := range cg.GeoRedundancy.TargetAvailabilityZones {
		details = append(details, GeoTargetDetail{
			Region: region,
			AZ:     target.AvailabilityZone,
			Status: target.Status,
		})
	}
	return details
}

func (cg *CgInfo) Replications() []RepTargetDetail {
	mode := cg.Replication.ReplicationMode
	if mode == "" {
		mode = "undef"
	}
	details := make([]RepTargetDetail, 0, len(cg.Replication.TargetAvailabilityZones))
	for _, target := range cg.Replication.TargetAvailabilityZones {
		details = append(details, RepTargetDetail{
			Mode:   mode,
			AZ:     target.AvailabilityZone,
			Status: target.Status,
		})
	}
	return details
}

func (cg *CgInfo) hasReplication() bool {
	return cg.Replication.ReplicationMode != "" || len(cg.Replication.TargetAvailabilityZones) > 0
}

func (cg *CgInfo) hasGeoRedundancy() bool {
	return cg.GeoRedundancy.Region != "" || len(cg.GeoRedundancy.TargetAvailabilityZones) > 0
}

type (
	GetAuthInfoer interface {
		GetAuthInfo(string) (*sgcp.AuthInfo, error)
	}

	logger interface {
		Debugf(format string, args ...any)
		Infof(format string, args ...any)
		Warnf(format string, args ...any)
		Errorf(format string, args ...any)
	}

	cgAPI interface {
		GetConsistencyGroup(ctx context.Context, uuid string) (method, url string, code int, data []byte, err error)
		PatchConsistencyGroup(ctx context.Context, uuid string, payload any) (method, url string, code int, data []byte, err error)
	}

	cgMgr struct {
		uuid  string
		log   logger
		api   cgAPI
		cache sgcp.CacheConfig
	}
)

// GetCg reads the consistency group past the cache. A cache that can not be
// dropped is an error: the read that follows would serve the entry GetCg was
// asked to go without, and the callers polling for a status change would spin
// on it until they time out.
func (m *cgMgr) GetCg(ctx context.Context) (*CgInfo, error) {
	if err := m.cacheClearGetCg(); err != nil {
		return nil, fmt.Errorf("clear the consistency group %s cache: %w", m.uuid, err)
	}
	return m.GetCachedCg(ctx)
}

func (m *cgMgr) GetCachedCg(ctx context.Context) (*CgInfo, error) {
	m.log.Debugf("get consistency group %s info", m.uuid)
	ts := time.Now()

	sig := m.cacheSig(cacheKeyGetCgInfo)
	ttl := time.Duration(m.cache.TTLSeconds) * time.Second
	o := ageingcache.NewOutputter(m.getCgOutputter(ctx))
	data, err := ageingcache.Output(o, sig, ttl)
	if err != nil {
		return nil, fmt.Errorf("get consistency group %s: %w", m.uuid, err)
	}
	var cg CgInfo
	if err := json.Unmarshal(data, &cg); err != nil {
		return nil, fmt.Errorf("unmarshal consistency group %s: %w", m.uuid, err)
	}
	m.log.Debugf("consistency group details: %+v (duration %.2f)", &cg, time.Since(ts).Seconds())
	return &cg, nil
}

func (m *cgMgr) getCgOutputter(ctx context.Context) func() ([]byte, error) {
	return func() ([]byte, error) {
		method, url, code, data, err := m.api.GetConsistencyGroup(ctx, m.uuid)
		if err != nil {
			return nil, err
		}
		if code != http.StatusOK {
			return nil, fmt.Errorf("get consistency group %s: unexpected status %d (method=%s url=%s)", m.uuid, code, method, url)
		}
		return data, nil
	}
}

func (m *cgMgr) cacheClearGetCg() error {
	return ageingcache.Clear(m.cacheSig(cacheKeyGetCgInfo))
}

func (m *cgMgr) Switchover(ctx context.Context, targetAZ string) error {
	payload := map[string]any{
		"operation": "switchover",
		"operationParameters": map[string]any{
			"availabilityZone": targetAZ,
		},
	}
	m.log.Infof("switchover consistency group %s to az %s ...", m.uuid, targetAZ)
	ts := time.Now()
	method, url, code, data, err := m.api.PatchConsistencyGroup(ctx, m.uuid, payload)
	if code == http.StatusPreconditionFailed {
		return fmt.Errorf("switchover consistency group %s: status %d (method=%s url=%s): %w", m.uuid, code, method, url, ErrPrecondition)
	}
	if err != nil {
		return err
	}
	if code != http.StatusAccepted {
		return fmt.Errorf("switchover consistency group %s: unexpected status %d (method=%s url=%s body=%s)", m.uuid, code, method, url, string(data))
	}
	m.log.Infof("| switched %s (duration %.2f)", m.uuid, time.Since(ts).Seconds())
	return nil
}

func (m *cgMgr) Failover(ctx context.Context, az string) error {
	payload := map[string]any{
		"operation": "failover",
		"operationParameters": map[string]any{
			"availabilityZone": az,
			"force":            true,
		},
	}
	m.log.Infof("failover consistency group %s to az %s ...", m.uuid, az)
	ts := time.Now()
	method, url, code, data, err := m.api.PatchConsistencyGroup(ctx, m.uuid, payload)
	if err != nil {
		return err
	}
	if code != http.StatusAccepted {
		return fmt.Errorf("failover consistency group %s: unexpected status %d (method=%s url=%s body=%s)", m.uuid, code, method, url, string(data))
	}
	m.log.Infof("| failover %s (duration %.2f)", m.uuid, time.Since(ts).Seconds())
	return nil
}

// ResumeReplication asks the provider to resume the replication of the
// consistency group. Unlike the switchover and the failover, the operation
// takes no availability zone: the group resumes toward the targets it is
// already configured with.
func (m *cgMgr) ResumeReplication(ctx context.Context) error {
	payload := map[string]any{"operation": "resume-replication"}
	m.log.Infof("resume-replication consistency group %s ...", m.uuid)
	ts := time.Now()
	method, url, code, data, err := m.api.PatchConsistencyGroup(ctx, m.uuid, payload)
	if err != nil {
		return err
	}
	if code != http.StatusAccepted {
		return fmt.Errorf("resume-replication consistency group %s: unexpected status %d (method=%s url=%s body=%s)", m.uuid, code, method, url, string(data))
	}
	m.log.Infof("| resume-replication %s (duration %.2f)", m.uuid, time.Since(ts).Seconds())
	return nil
}

func (m *cgMgr) cacheSig(name string) string {
	return fmt.Sprintf("%s:%s", name, m.uuid)
}

type T struct {
	resource.T

	UUID     string        `json:"uuid"`
	AZ       string        `json:"az,omitempty"`
	Secret   string        `json:"secret,omitempty"`
	Endpoint string        `json:"endpoint,omitempty"`
	Timeout  time.Duration `json:"timeout"`
	Failover bool          `json:"failover"`

	lastWaitMsg time.Time
	mgr         *cgMgr
	authInfoer  GetAuthInfoer
}

func New() resource.Driver {
	return &T{}
}

func (t *T) Configure() error {
	cfg := sgcp.GetConfig()
	if cfg == nil {
		return fmt.Errorf("mandatory config file is required: %s", sgcp.DefaultConfigPath)
	}
	if t.Secret == "" {
		t.Secret = cfg.Auth.DefaultSecret
	}
	if t.Secret == "" {
		return fmt.Errorf("secret is required (neither defined into secret keyword nor config file %s", sgcp.DefaultConfigPath)
	}
	cfg = cfg.WithAuthSecret(t.Secret)
	if t.Endpoint == "" {
		t.Endpoint = cfg.Files.BaseURL
	}
	if t.Endpoint == "" {
		return fmt.Errorf("file endpoint is required (neither defined into endpoint keyword nor config file %s", sgcp.DefaultConfigPath)
	}
	cfg = cfg.WithFileURL(t.Endpoint)
	if t.AZ == "" {
		// The az keyword defaults to {node.labels.az}, which evaluates to
		// an empty string on a node where that label is not set. Refuse
		// now: without a local az, start would ask for a switchover to
		// the "" availability zone.
		return fmt.Errorf("az is required (neither defined into the az keyword nor by the node az label)")
	}
	return t.configureMgr(cfg)
}

func (t *T) configureMgr(cfg *sgcp.Config) error {
	httpClient, err := httpclientcache.Client(httpclientcache.Options{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	if t.authInfoer == nil {
		t.authInfoer = &sgcphelper.GetAuthInfoFromDatastorePather{}
	}
	authInfo, err := t.authInfoer.GetAuthInfo(t.Secret)
	if err != nil {
		return fmt.Errorf("get auth info: %w", err)
	}
	tk := sgcp.NewTokenFactory(t.Log(), httpClient, &cfg.Auth, authInfo)
	t.mgr = &cgMgr{
		uuid:  t.UUID,
		log:   t.Log(),
		api:   sgcp.NewFilesAPI(cfg, httpClient, t.Log(), tk),
		cache: cfg.Cache,
	}
	return nil
}

func (t *T) Label(_ context.Context) string {
	return t.UUID
}

func (t *T) logGeoRedundancies(cg *CgInfo) {
	geos := cg.GeoRedundancies()
	if len(geos) == 0 {
		return
	}
	geoStates := map[string]struct{}{}
	for _, g := range geos {
		geoStates[g.Status] = struct{}{}
	}
	switch cg.Status {
	case "passive":
		t.StatusLog().Info("geo mode remote -> local")
	case "ready":
		if cg.AvailabilityZone == t.AZ {
			switch {
			case isOnlyStatus(geoStates, "replicated"):
				t.StatusLog().Info("geo mode local -> remote")
			case isOnlyStatus(geoStates, "broken"):
				t.StatusLog().Info("geo mode failover on remote")
			case isOnlyStatus(geoStates, "unknown"):
				t.StatusLog().Warn("geo mode failover on local ?")
			default:
				t.StatusLog().Info("geo mode transitioning states %s", joinStates(geoStates))
			}
		} else {
			t.StatusLog().Info("geo mode remote -> remote")
		}
	default:
		t.StatusLog().Warn("geo mode states '%s'", joinStates(geoStates))
	}
	t.StatusLog().Info("geo local az %s", t.AZ)
	regions := map[string]struct{}{}
	for _, target := range geos {
		regions[target.Region] = struct{}{}
	}
	for _, region := range sortedKeys(regions) {
		t.StatusLog().Info("geo remote region %s", region)
	}
	for _, target := range geos {
		if target.Status == "replicated" {
			t.StatusLog().Info("geo %s %s", target.AZ, target.Status)
		} else {
			t.StatusLog().Warn("geo %s %s", target.AZ, target.Status)
		}
	}
}

func (t *T) logReplications(cg *CgInfo) {
	reps := cg.Replications()
	if len(reps) == 0 {
		return
	}
	mode := cg.Replication.ReplicationMode
	if mode == "" {
		mode = "undef"
	}
	switch {
	case cg.AvailabilityZone == t.AZ:
		t.StatusLog().Info("rep mode %s local -> remote", mode)
	case repTargetsContainAZ(reps, t.AZ):
		t.StatusLog().Info("rep mode %s remote -> local", mode)
	default:
		t.StatusLog().Info("rep mode %s", mode)
	}
	for _, target := range reps {
		where := "remote"
		if target.AZ == t.AZ {
			where = "local"
		}
		if target.Status == "replicated" {
			t.StatusLog().Info("rep %s %s %s", where, target.AZ, target.Status)
		} else {
			t.StatusLog().Warn("rep %s %s %s", where, target.AZ, target.Status)
		}
	}
}

func (t *T) Status(ctx context.Context) status.T {
	if disabled, err := sgcp.IsDisabled(rawconfig.NodeVarDir()); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.NotApplicable
	} else if disabled {
		t.StatusLog().Info("xaas status disabled")
		return status.NotApplicable
	}
	cg, err := t.mgr.GetCachedCg(ctx)
	if err != nil {
		t.StatusLog().Warn("get consistency group: %s", err)
		return status.NotApplicable
	}
	if cg.Status == "ready" || cg.Status == "passive" {
		t.StatusLog().Info("status %s %s", cg.Status, cg.AvailabilityZone)
	} else {
		t.StatusLog().Warn("status %s %s", cg.Status, cg.AvailabilityZone)
	}
	t.logGeoRedundancies(cg)
	t.logReplications(cg)
	return status.NotApplicable
}

func (t *T) waitStatus(ctx context.Context, expectedStates []string) error {
	t.lastWaitMsg = time.Time{}
	fn := func() (bool, error) {
		cg, err := t.mgr.GetCg(ctx)
		if err != nil {
			return false, err
		}
		now := time.Now()
		if contains(expectedStates, cg.Status) {
			t.Log().Infof("| consistency group %s status is now %s", t.UUID, cg.Status)
			return true, nil
		}
		if strings.Contains(cg.Status, "failed") || strings.Contains(cg.Status, "rollback") {
			msg := fmt.Sprintf("abort waiting for consistency group %s status in '%v' because found status %s",
				t.UUID, expectedStates, cg.Status)
			t.Log().Warnf("%s", msg)
			return false, errors.New(msg)
		}
		if now.Sub(t.lastWaitMsg) >= waitMsgInterval {
			t.Log().Infof("| waiting for consistency group %s status in %v. current status is %s",
				t.UUID, expectedStates, cg.Status)
			t.lastWaitMsg = now
		}
		return false, nil
	}
	errMsg := fmt.Sprintf("timeout waiting for consistency group %s status in %v", t.UUID, expectedStates)
	return t.waitForFn(ctx, fn, t.Timeout, retryWaitDelay, errMsg)
}

func (t *T) waitStatusAndAZ(ctx context.Context, expectedStates []string, expectedAZ string) error {
	t.lastWaitMsg = time.Time{}
	fn := func() (bool, error) {
		cg, err := t.mgr.GetCg(ctx)
		if err != nil {
			return false, err
		}
		now := time.Now()
		if contains(expectedStates, cg.Status) {
			if expectedAZ != "" && cg.AvailabilityZone != expectedAZ {
				msg := fmt.Sprintf("consistency group %s reached status %s but in availability zone %s (expected %s)",
					t.UUID, cg.Status, cg.AvailabilityZone, expectedAZ)
				t.Log().Warnf("%s", msg)
				// Continue waiting; the AZ may still be transitioning.
				return false, nil
			}
			t.Log().Infof("| consistency group %s status is now %s", t.UUID, cg.Status)
			return true, nil
		}
		if strings.Contains(cg.Status, "failed") || strings.Contains(cg.Status, "rollback") {
			msg := fmt.Sprintf("abort waiting for consistency group %s status in '%v' because found status %s",
				t.UUID, expectedStates, cg.Status)
			t.Log().Warnf("%s", msg)
			return false, errors.New(msg)
		}
		if now.Sub(t.lastWaitMsg) >= waitMsgInterval {
			t.Log().Infof("| waiting for consistency group %s status in %v and availability zone %s. current status is %s (az %s)",
				t.UUID, expectedStates, expectedAZ, cg.Status, cg.AvailabilityZone)
			t.lastWaitMsg = now
		}
		return false, nil
	}
	errMsg := fmt.Sprintf("timeout waiting for consistency group %s status in %v and availability zone %s", t.UUID, expectedStates, expectedAZ)
	return t.waitForFn(ctx, fn, t.Timeout, retryWaitDelay, errMsg)
}

func (t *T) waitForFn(ctx context.Context, fn func() (bool, error), timeout, retryDelay time.Duration, errMsg string) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := fn()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New(errMsg)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
}

func (t *T) waitReady(ctx context.Context) error {
	return t.waitStatus(ctx, []string{"ready", "passive"})
}

func (t *T) Start(ctx context.Context) error {
	return t.start(ctx)
}

func (t *T) start(ctx context.Context) error {
	// An unknown disabled state has to stop the action here. Reading it as
	// disabled would return success without switching the consistency group
	// over, and the instance would be declared up in the wrong az.
	disabled, err := sgcp.IsDisabled(rawconfig.NodeVarDir())
	if err != nil {
		return err
	}
	if disabled {
		t.Log().Infof("skip start of consistency group %s: sgcp support disabled", t.UUID)
		return nil
	}
	cg, err := t.mgr.GetCg(ctx)
	if err != nil {
		return err
	}
	if !contains([]string{"ready", "passive"}, cg.Status) {
		t.Log().Infof("consistency group %s has an operation in progress. waiting ...", t.UUID)
		if err := t.waitReady(ctx); err != nil {
			return err
		}
		cg, err = t.mgr.GetCachedCg(ctx)
		if err != nil {
			return err
		}
	}
	if cg.Status == "ready" && cg.AvailabilityZone == t.AZ {
		t.Log().Infof("consistency group %s is already up", t.UUID)
		return nil
	}
	if actioncontext.IsForce(ctx) {
		if err := t.mgr.Failover(ctx, t.AZ); err != nil {
			return err
		}
	} else if err := t.mgr.Switchover(ctx, t.AZ); err != nil {
		if !errors.Is(err, ErrPrecondition) {
			return err
		}
		msg := fmt.Sprintf("consistency group %s switchover 412 error", t.UUID)
		if !t.Failover {
			t.Log().Errorf("%s, skip failover fallback (resource failover is False)", msg)
			return err
		}
		// A failover is not a switchover: it is only tried on its own for an
		// orchestration the daemon drives. An operator asks for it with
		// --force, which does not come here.
		if !env.HasDaemonOrigin() {
			t.Log().Errorf("%s, skip failover fallback, use --force if you want to try failover", msg)
			return err
		}
		t.Log().Infof("%s, try failover", msg)
		if err := t.mgr.Failover(ctx, t.AZ); err != nil {
			return err
		}
	}
	return t.waitStatusAndAZ(ctx, []string{"ready"}, t.AZ)
}

// Stop does not move the consistency group. It stays in the availability zone
// the last start switched it to, and the start of the instance elsewhere is
// what moves it. The action says so rather than passing silently, so that an
// operator watching a stop is not left wondering whether it did anything.
//
// Having nothing to undo, it also treats an undecidable disabled flag as a
// warning where start has to refuse to go on.
func (t *T) Stop(ctx context.Context) error {
	_ = ctx
	disabled, err := sgcp.IsDisabled(rawconfig.NodeVarDir())
	switch {
	case err != nil:
		t.Log().Warnf("%s", err)
	case disabled:
		t.Log().Infof("skip stop of consistency group %s: sgcp support disabled", t.UUID)
	default:
		t.Log().Infof("stop leaves consistency group %s where it is, a start elsewhere moves it", t.UUID)
	}
	return nil
}

// Resync implements the resource.Resyncer interface and is the entry point
// for the sync-resync action.
func (t *T) Resync(ctx context.Context) error {
	return t.SyncResume(ctx)
}

// SyncResume performs the actual resume-replication logic and is kept for
// backward compatibility with existing callers/tests.
func (t *T) SyncResume(ctx context.Context) error {
	t.Log().Infof("sync resume ...")
	if err := t.syncResume(ctx); err != nil {
		t.Log().Errorf("sync resume failed")
		return err
	}
	t.Log().Infof("sync resume succeed")
	return nil
}

func (t *T) syncResume(ctx context.Context) error {
	msgPrefix := fmt.Sprintf("consistency group %s", t.UUID)
	pendingResume := false
	cg, err := t.mgr.GetCg(ctx)
	if err != nil {
		return err
	}
	if err := t.checkResumable(cg); err != nil {
		switch {
		case errors.Is(err, ErrAlreadyResumed):
			t.Log().Infof("%s doesn't require sync resume", msgPrefix)
			return nil
		case errors.Is(err, ErrResumeInProgress):
			pendingResume = true
		default:
			return err
		}
	}
	if !pendingResume {
		if err := t.mgr.ResumeReplication(ctx); err != nil {
			return err
		}
	}
	if err := t.waitStatus(ctx, []string{"ready", "passive"}); err != nil {
		return err
	}
	cg, err = t.mgr.GetCachedCg(ctx)
	if err != nil {
		return err
	}
	if err := t.checkResumable(cg); err != nil {
		if errors.Is(err, ErrAlreadyResumed) {
			t.Log().Infof("%s now resumed", msgPrefix)
			return nil
		}
		return fmt.Errorf("%s still not resumed: %w", msgPrefix, err)
	}
	// The group accepts a resume again, which is not the resumed state the
	// operation was waiting for.
	return fmt.Errorf("%s still not resumed", msgPrefix)
}

func (t *T) checkResumable(cg *CgInfo) error {
	hasRep := cg.hasReplication()
	hasGeo := cg.hasGeoRedundancy()

	switch {
	case hasRep && hasGeo:
		return t.checkResumableReplicationAndGeo(cg)
	case hasRep:
		return t.checkResumableReplicationOnly(cg)
	case hasGeo:
		return t.checkResumableGeoOnly(cg)
	default:
		return fmt.Errorf("sync resume not allowed on cg %s without replication or georedundancy", t.UUID)
	}
}

func (t *T) checkResumableReplicationAndGeo(cg *CgInfo) error {
	// Reject explicitly forbidden in-progress or failed operations
	switch cg.Status {
	case "failover", "failed", "rollback":
		return fmt.Errorf("sync resume not allowed on cg %s in status %s", t.UUID, cg.Status)
	}

	localRepStatus := t.localRepStatus(cg)
	if localRepStatus == "" {
		return fmt.Errorf("sync resume not allowed on cg %s: no local replication target found", t.UUID)
	}
	if !contains([]string{"unknown", "replicated", "replicating"}, localRepStatus) {
		return fmt.Errorf("sync resume not allowed on cg %s where status is %s and local replication status is %s",
			t.UUID, cg.Status, localRepStatus)
	}

	geos := cg.GeoRedundancies()
	if len(geos) == 0 {
		return fmt.Errorf("sync resume not allowed on cg %s: georedundancy has no target availability zones", t.UUID)
	}
	allGeoReplicated := true
	for _, g := range geos {
		if g.Status != "replicated" {
			allGeoReplicated = false
			break
		}
	}

	// Already resumed if local replication is passive/replicated and all geo targets are replicated
	if contains([]string{"passive", "replicated"}, localRepStatus) && allGeoReplicated {
		return ErrAlreadyResumed
	}
	if cg.Status == "resuming" {
		return ErrResumeInProgress
	}
	return nil
}

func (t *T) checkResumableReplicationOnly(cg *CgInfo) error {
	localRepStatus := t.localRepStatus(cg)

	if !contains([]string{"ready", "resuming"}, cg.Status) {
		return fmt.Errorf("sync resume not allowed when cg %s status is %s", t.UUID, cg.Status)
	}
	if cg.AvailabilityZone == t.AZ {
		return fmt.Errorf("sync resume not allowed on cg %s where cg az is local az", t.UUID)
	}
	if localRepStatus == "replicated" {
		return ErrAlreadyResumed
	}
	if cg.Status == "resuming" {
		return ErrResumeInProgress
	}
	if localRepStatus != "unknown" {
		return fmt.Errorf("sync resume not allowed on cg %s where local replication status is %s", t.UUID, localRepStatus)
	}
	return nil
}

func (t *T) checkResumableGeoOnly(cg *CgInfo) error {
	if cg.Status == "passive" {
		return ErrAlreadyResumed
	}
	if cg.Status == "resuming" {
		return ErrResumeInProgress
	}
	if cg.Status != "ready" {
		return fmt.Errorf("sync resume not allowed when cg %s status is %s", t.UUID, cg.Status)
	}
	geos := cg.GeoRedundancies()
	if len(geos) == 0 {
		return fmt.Errorf("sync resume not allowed on cg %s: georedundancy has no target availability zones", t.UUID)
	}
	for _, g := range geos {
		if !contains([]string{"broken", "unknown"}, g.Status) {
			return fmt.Errorf("sync resume not allowed on '%s' cg %s where georedundancy status is '%s'", cg.Status, t.UUID, g.Status)
		}
	}
	return nil
}

func (t *T) localRepStatus(cg *CgInfo) string {
	var localRep []RepTargetDetail
	for _, rep := range cg.Replications() {
		if rep.AZ == t.AZ {
			localRep = append(localRep, rep)
		}
	}
	if len(localRep) == 1 {
		return localRep[0].Status
	}
	// If multiple local targets (unusual) return the first non-empty status
	for _, rep := range localRep {
		if rep.Status != "" {
			return rep.Status
		}
	}
	if cg.AvailabilityZone == t.AZ {
		return cg.Status
	}
	return ""
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func isOnlyStatus(set map[string]struct{}, val string) bool {
	if len(set) != 1 {
		return false
	}
	_, ok := set[val]
	return ok
}

func joinStates(set map[string]struct{}) string {
	return strings.Join(sortedKeys(set), ",")
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func repTargetsContainAZ(targets []RepTargetDetail, az string) bool {
	for _, target := range targets {
		if target.AZ == az {
			return true
		}
	}
	return false
}
