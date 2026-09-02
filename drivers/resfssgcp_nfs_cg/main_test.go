package resfssgcp_nfs_cg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/sgcp"
	"github.com/opensvc/om3/v3/util/sgcpcgtesthelper"
	"github.com/opensvc/om3/v3/util/testsgcphelper"
)

func setup(t *testing.T) func() {
	t.Helper()
	cfgFile := testsgcphelper.InstallConfig(t)
	sgcp.SetConfigForTest(cfgFile)
	require.NotNil(t, sgcp.GetConfig())
	defaultRetryWaitDelay := retryWaitDelay
	retryWaitDelay = 150 * time.Millisecond
	return func() {
		sgcp.SetConfigForTest("")
		retryWaitDelay = defaultRetryWaitDelay
	}
}

const (
	cgUUIDRegion1 = "cg-uuid-region1"
	region1AZ1    = "region1-az1"
	region1AZ2    = "region1-az2"
	region2AZ1    = "region2-az1"
)

const repCgAZ1Active = `{
	"availabilityZone": "region1-az1",
	"replication": {
		"replicationMode": "sync",
		"targetAvailabilityZones": [
			{"availabilityZone": "region1-az2", "status": "replicated"}
		]
	},
	"status": "ready",
	"uuid": "cg-uuid-region1"
}`

const repCgAZ2Active = `{
	"availabilityZone": "region1-az2",
	"replication": {
		"replicationMode": "sync",
		"targetAvailabilityZones": [
			{"availabilityZone": "region1-az1", "status": "replicated"}
		]
	},
	"status": "ready",
	"uuid": "cg-uuid-region1"
}`

const repCgAZ1FailoverInProgressToAZ2 = `{
	"availabilityZone": "region1-az1",
	"replication": {
		"replicationMode": "sync",
		"targetAvailabilityZones": [
			{"availabilityZone": "region1-az2", "status": "replicated"}
		]
	},
	"status": "failover",
	"uuid": "cg-uuid-region1"
}`

const repCgAZ1ResumingRemoteAZ2 = `{
	"availabilityZone": "region1-az1",
	"replication": {
		"replicationMode": "sync",
		"targetAvailabilityZones": [
			{"availabilityZone": "region1-az2", "status": "unknown"}
		]
	},
	"status": "resuming",
	"uuid": "cg-uuid-region1"
}`

const geoCgRegion1AZ1Active = `{
	"availabilityZone": "region1-az1",
	"georedundancy": {
		"region": "region2",
		"targetAvailabilityZones": [
			{"availabilityZone": "region2-az1", "status": "replicated"}
		],
		"uuid": "cg-uuid-region2"
	},
	"status": "ready",
	"uuid": "cg-uuid-region1"
}`

const geoCgRegion1AZ1Passive = `{
	"availabilityZone": "region1-az1",
	"georedundancy": {
		"region": "region2",
		"targetAvailabilityZones": [
			{"availabilityZone": "region2-az1", "status": "replicated"}
		],
		"uuid": "cg-uuid-region2"
	},
	"status": "passive",
	"uuid": "cg-uuid-region1"
}`

const mixedRepGeoRegion1AZ1Active = `{
	"availabilityZone": "region1-az1",
	"georedundancy": {
		"region": "region2",
		"targetAvailabilityZones": [
			{"availabilityZone": "region2-az1", "status": "replicated"}
		],
		"uuid": "cg-uuid-region2"
	},
	"replication": {
		"replicationMode": "sync",
		"targetAvailabilityZones": [
			{"availabilityZone": "region1-az2", "status": "replicated"}
		]
	},
	"status": "ready",
	"uuid": "cg-uuid-region1"
}`

func mustParseCg(t *testing.T, raw string) *CgInfo {
	t.Helper()
	var cg CgInfo
	if err := json.Unmarshal([]byte(raw), &cg); err != nil {
		t.Fatalf("unmarshal fixture: %s", err)
	}
	return &cg
}

func TestCgInfo_Replications(t *testing.T) {
	cg := mustParseCg(t, repCgAZ1Active)
	reps := cg.Replications()
	if len(reps) != 1 {
		t.Fatalf("expected 1 replication target, got %d", len(reps))
	}
	if reps[0].AZ != region1AZ2 || reps[0].Status != "replicated" || reps[0].Mode != "sync" {
		t.Fatalf("unexpected replication target: %+v", reps[0])
	}
	if !cg.hasReplication() {
		t.Fatal("expected hasReplication() to be true")
	}
	if cg.hasGeoRedundancy() {
		t.Fatal("expected hasGeoRedundancy() to be false")
	}
}

func TestCgInfo_GeoRedundancies(t *testing.T) {
	cg := mustParseCg(t, geoCgRegion1AZ1Active)
	geos := cg.GeoRedundancies()
	if len(geos) != 1 {
		t.Fatalf("expected 1 geo-redundancy target, got %d", len(geos))
	}
	if geos[0].AZ != region2AZ1 || geos[0].Status != "replicated" || geos[0].Region != "region2" {
		t.Fatalf("unexpected geo-redundancy target: %+v", geos[0])
	}
	if !cg.hasGeoRedundancy() {
		t.Fatal("expected hasGeoRedundancy() to be true")
	}
	if cg.hasReplication() {
		t.Fatal("expected hasReplication() to be false")
	}
}

func TestCgInfo_Mixed(t *testing.T) {
	cg := mustParseCg(t, mixedRepGeoRegion1AZ1Active)
	if !cg.hasReplication() || !cg.hasGeoRedundancy() {
		t.Fatalf("expected both replication and geo-redundancy, got hasRep=%v hasGeo=%v",
			cg.hasReplication(), cg.hasGeoRedundancy())
	}
}

func TestConfigure_RequiresAZ(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	// The az keyword defaults to {node.labels.az}, which evaluates to an
	// empty string on a node where the label is not set.
	drv := &T{UUID: cgUUIDRegion1}
	err := drv.Configure()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "az is required")
}

func TestCheckResumable_NotSyncable(t *testing.T) {
	tests := []struct {
		name    string
		az      string
		raw     string
		wantMsg string
	}{
		{
			name:    "replication active az",
			az:      region1AZ1,
			raw:     repCgAZ1Active,
			wantMsg: "sync resume not allowed on cg cg-uuid-region1 where cg az is local az",
		},
		{
			name:    "replication active az while failover in progress",
			az:      region1AZ1,
			raw:     repCgAZ1FailoverInProgressToAZ2,
			wantMsg: "sync resume not allowed when cg cg-uuid-region1 status is failover",
		},
		{
			name:    "replication active az while resuming",
			az:      region1AZ1,
			raw:     repCgAZ1ResumingRemoteAZ2,
			wantMsg: "sync resume not allowed on cg cg-uuid-region1 where cg az is local az",
		},
		{
			name:    "georedundancy active az region az and status is not broken",
			az:      region1AZ1,
			raw:     geoCgRegion1AZ1Active,
			wantMsg: "sync resume not allowed on 'ready' cg cg-uuid-region1 where georedundancy status is 'replicated'",
		},
		{
			name: "cg is ready and mix replication and georedundancy",
			az:   region1AZ1,
			raw:  mixedRepGeoRegion1AZ1Active,
			wantMsg: "sync resume not allowed on cg cg-uuid-region1 where status is ready and local replication" +
				" status is ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cg := mustParseCg(t, tt.raw)
			rt := &T{UUID: cgUUIDRegion1, AZ: tt.az}

			err := rt.checkResumable(cg)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if errors.Is(err, ErrAlreadyResumed) || errors.Is(err, ErrResumeInProgress) {
				t.Fatalf("expected a plain error, got sentinel: %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestCheckResumable_AlreadyResumed(t *testing.T) {
	tests := []struct {
		name string
		az   string
		raw  string
	}{
		{name: "replication called from non active az", az: region1AZ1, raw: repCgAZ2Active},
		{name: "geo called from passive region", az: region1AZ1, raw: geoCgRegion1AZ1Passive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cg := mustParseCg(t, tt.raw)
			rt := &T{UUID: cgUUIDRegion1, AZ: tt.az}

			err := rt.checkResumable(cg)
			if !errors.Is(err, ErrAlreadyResumed) {
				t.Fatalf("expected ErrAlreadyResumed, got %v", err)
			}
		})
	}
}

func TestLocalRepStatus(t *testing.T) {
	cg := mustParseCg(t, repCgAZ2Active)
	rt := &T{UUID: cgUUIDRegion1, AZ: region1AZ1}
	if got := rt.localRepStatus(cg); got != "replicated" {
		t.Fatalf("localRepStatus = %q, want %q", got, "replicated")
	}

	rt2 := &T{UUID: cgUUIDRegion1, AZ: "region3-az1"}
	if got := rt2.localRepStatus(cg); got != "" {
		t.Fatalf("localRepStatus = %q, want empty", got)
	}
}

func TestContains(t *testing.T) {
	if !contains([]string{"ready", "passive"}, "ready") {
		t.Fatal("expected contains to find 'ready'")
	}
	if contains([]string{"ready", "passive"}, "resuming") {
		t.Fatal("expected contains to not find 'resuming'")
	}
}

func TestIsOnlyStatus(t *testing.T) {
	set := map[string]struct{}{"replicated": {}}
	if !isOnlyStatus(set, "replicated") {
		t.Fatal("expected isOnlyStatus to be true for a single matching entry")
	}
	set["broken"] = struct{}{}
	if isOnlyStatus(set, "replicated") {
		t.Fatal("expected isOnlyStatus to be false once a second state is present")
	}
}

func TestJoinStates(t *testing.T) {
	set := map[string]struct{}{"broken": {}, "unknown": {}}
	if got := joinStates(set); got != "broken,unknown" {
		t.Fatalf("joinStates = %q, want %q (sorted)", got, "broken,unknown")
	}
}

func TestRepTargetsContainAZ(t *testing.T) {
	targets := []RepTargetDetail{{AZ: region1AZ2, Status: "replicated"}}
	if !repTargetsContainAZ(targets, region1AZ2) {
		t.Fatal("expected repTargetsContainAZ to find region1-az2")
	}
	if repTargetsContainAZ(targets, region1AZ1) {
		t.Fatal("expected repTargetsContainAZ to not find region1-az1")
	}
}

func TestWaitForFn_SucceedsBeforeTimeout(t *testing.T) {
	rt := &T{}
	calls := 0
	fn := func() (bool, error) {
		calls++
		return calls >= 3, nil
	}
	ctx := context.Background()
	err := rt.waitForFn(ctx, fn, time.Second, time.Millisecond, "timed out")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWaitForFn_TimesOut(t *testing.T) {
	rt := &T{}
	fn := func() (bool, error) { return false, nil }
	ctx := context.Background()
	err := rt.waitForFn(ctx, fn, 5*time.Millisecond, time.Millisecond, "timed out waiting")
	if err == nil || !strings.Contains(err.Error(), "timed out waiting") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitForFn_PropagatesError(t *testing.T) {
	rt := &T{}
	boom := errors.New("boom")
	fn := func() (bool, error) { return false, boom }
	ctx := context.Background()
	err := rt.waitForFn(ctx, fn, time.Second, time.Millisecond, "timed out")
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func setupMockCG(t *testing.T, entries []sgcpcgtesthelper.CgEntry) (*sgcpcgtesthelper.DB, *sgcpcgtesthelper.API) {
	t.Helper()
	db := sgcpcgtesthelper.NewDB()
	db.Setup(entries)
	api := sgcpcgtesthelper.NewAPI(db)
	return db, api
}

func newTestDriver(t *testing.T, id, az string, timeout time.Duration, failover bool, api *sgcpcgtesthelper.API) *T {
	t.Helper()
	drv := &T{
		UUID:     id,
		AZ:       az,
		Timeout:  timeout,
		Failover: failover,
	}
	drv.mgr = &cgMgr{
		uuid:  id,
		log:   drv.Log(),
		api:   api,
		cache: sgcp.CacheConfig{TTLSeconds: 1},
	}
	return drv
}

func TestStart_SwitchoverSuccess(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
			Replication: sgcpcgtesthelper.ReplicationInfo{
				ReplicationMode: "sync",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ1, Status: "replicated"},
				},
			},
		},
	})
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)

	ctx := context.Background()
	err := drv.Start(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	assert.Equal(t, region1AZ1, entry.AvailabilityZone)
	assert.Equal(t, "ready", entry.Status)

	calls := db.CallCounts()
	assert.Equal(t, 2, calls.Get)
	assert.Equal(t, 1, calls.Patch)
	assert.Equal(t, 1, calls.Switch)
	assert.Equal(t, 0, calls.Fail)
}

func TestStart_Switchover412_FailoverAllowed(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
		},
	})
	db.PatchSwitchoverFunc = func(ctx context.Context, u, targetAZ string) error {
		return fmt.Errorf("simulated precondition failed: %w", ErrPrecondition)
	}
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, true, api)
	t.Setenv(env.ActionOriginVar, string(env.ActionOriginDaemonMonitor))

	ctx := context.Background()
	err := drv.Start(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	assert.Equal(t, region1AZ1, entry.AvailabilityZone)
	assert.Equal(t, "ready", entry.Status)

	calls := db.CallCounts()
	assert.Equal(t, 2, calls.Get)
	assert.Equal(t, 2, calls.Patch)
	assert.Equal(t, 1, calls.Switch)
	assert.Equal(t, 1, calls.Fail)
}

func TestStart_Switchover412_FailoverNotAllowed_NoDaemon(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()
	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
		},
	})
	db.PatchSwitchoverFunc = func(ctx context.Context, u, targetAZ string) error {
		return fmt.Errorf("simulated precondition failed: %w", ErrPrecondition)
	}
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)
	t.Setenv(env.ActionOriginVar, string(env.ActionOriginUser))

	ctx := context.Background()
	err := drv.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrecondition))

	calls := db.CallCounts()
	assert.Equal(t, 1, calls.Get)
	assert.Equal(t, 1, calls.Patch)
	assert.Equal(t, 1, calls.Switch)
	assert.Equal(t, 0, calls.Fail)
}

func TestStart_ForceFailover(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
		},
	})
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)

	ctx := actioncontext.WithForce(context.Background(), true)
	err := drv.Start(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	assert.Equal(t, region1AZ1, entry.AvailabilityZone)
	assert.Equal(t, "ready", entry.Status)

	calls := db.CallCounts()
	assert.Equal(t, 2, calls.Get)
	assert.Equal(t, 1, calls.Patch)
	assert.Equal(t, 0, calls.Switch)
	assert.Equal(t, 1, calls.Fail)
}

func TestStart_AlreadyUp(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ1,
			Status:           "ready",
		},
	})
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)

	ctx := context.Background()
	err := drv.Start(ctx)
	assert.NoError(t, err)

	calls := db.CallCounts()
	assert.Equal(t, 1, calls.Get)
	assert.Equal(t, 0, calls.Patch)
}

func TestStart_OperationInProgress_WaitReady(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ1,
			Status:           "failover",
		},
	})
	go func() {
		time.Sleep(100 * time.Millisecond)
		entry, _ := db.Search(id)
		entry.Status = "ready"
		_ = db.Update(entry)
	}()

	drv := newTestDriver(t, id, region1AZ1, 2*time.Second, false, api)
	ctx := context.Background()
	err := drv.Start(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	assert.Equal(t, "ready", entry.Status)
	assert.Equal(t, region1AZ1, entry.AvailabilityZone)

	calls := db.CallCounts()
	assert.GreaterOrEqual(t, calls.Get, 2)
	assert.Equal(t, 0, calls.Patch)
}

func TestSyncResume_ReplicationOnly_Success(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
			Replication: sgcpcgtesthelper.ReplicationInfo{
				ReplicationMode: "sync",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ1, Status: "unknown"},
				},
			},
		},
	})

	db.PatchResumeFunc = func(ctx context.Context, u string) error {
		entry, _ := db.Search(u)
		entry.Replication.TargetAvailabilityZones[0].Status = "replicated"
		entry.Status = "ready"
		_ = db.Update(entry)
		return nil
	}

	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)
	ctx := context.Background()
	err := drv.SyncResume(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	var localStatus string
	for _, rep := range entry.Replication.TargetAvailabilityZones {
		if rep.AvailabilityZone == region1AZ1 {
			localStatus = rep.Status
			break
		}
	}
	assert.Equal(t, "replicated", localStatus)
	assert.Equal(t, "ready", entry.Status)

	calls := db.CallCounts()
	assert.Equal(t, 2, calls.Get)
	assert.Equal(t, 1, calls.Patch)
	assert.Equal(t, 1, calls.Resume)
}

// TestSyncResume_ReportsTheReason verifies a resume that does not take is
// reported with what checkResumable said about the group, and not with the
// bare "still not resumed" it used to return, the reason going to the log
// alone.
func TestSyncResume_ReportsTheReason(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
			Replication: sgcpcgtesthelper.ReplicationInfo{
				ReplicationMode: "sync",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ1, Status: "unknown"},
				},
			},
		},
	})

	// The group settles in a state a resume is not allowed from.
	db.PatchResumeFunc = func(ctx context.Context, u string) error {
		entry, _ := db.Search(u)
		entry.Status = "passive"
		_ = db.Update(entry)
		return nil
	}

	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)
	ctx := context.Background()

	err := drv.SyncResume(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still not resumed")
	assert.Containsf(t, err.Error(), "status is passive", "the returned error drops the reason: %s", err)
}

func TestSyncResume_AlreadyResumed(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
			Replication: sgcpcgtesthelper.ReplicationInfo{
				ReplicationMode: "sync",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ1, Status: "replicated"},
				},
			},
		},
	})
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)

	ctx := context.Background()
	err := drv.SyncResume(ctx)
	assert.NoError(t, err)

	calls := db.CallCounts()
	assert.Equal(t, 1, calls.Get)
	assert.Equal(t, 0, calls.Patch)
}

func TestSyncResume_ResumeInProgress(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "resuming",
			Replication: sgcpcgtesthelper.ReplicationInfo{
				ReplicationMode: "sync",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ1, Status: "unknown"},
				},
			},
		},
	})

	var mu sync.Mutex
	getCount := 0

	api.GetConsistencyGroupFunc = func(ctx context.Context, u string) (method, url string, code int, data []byte, err error) {
		mu.Lock()
		getCount++
		if getCount >= 2 {
			entry, _ := db.Search(u)
			entry.Status = "ready"
			entry.Replication.TargetAvailabilityZones[0].Status = "replicated"
			_ = db.Update(entry)
		}
		mu.Unlock()

		savedFunc := api.GetConsistencyGroupFunc
		api.GetConsistencyGroupFunc = nil
		defer func() { api.GetConsistencyGroupFunc = savedFunc }()

		return api.GetConsistencyGroup(ctx, u)
	}

	drv := newTestDriver(t, id, region1AZ1, 2*time.Second, false, api)
	ctx := context.Background()
	err := drv.SyncResume(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	assert.Equal(t, "ready", entry.Status)
	localStatus := entry.Replication.TargetAvailabilityZones[0].Status
	assert.Equal(t, "replicated", localStatus)

	calls := db.CallCounts()
	assert.GreaterOrEqual(t, calls.Get, 2)
	assert.Equal(t, 0, calls.Patch)
	assert.Equal(t, 0, calls.Resume)
}

func TestSyncResume_GeoOnly_Success(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()

	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ2,
			Status:           "ready",
			GeoRedundancy: sgcpcgtesthelper.GeoRedundancyInfo{
				Region: "region2",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ1, Status: "broken"},
				},
			},
		},
	})
	db.PatchResumeFunc = func(ctx context.Context, u string) error {
		entry, _ := db.Search(u)
		entry.GeoRedundancy.TargetAvailabilityZones[0].Status = "replicated"
		entry.Status = "passive"
		_ = db.Update(entry)
		return nil
	}

	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)

	ctx := context.Background()
	err := drv.SyncResume(ctx)
	assert.NoError(t, err)

	entry, ok := db.Search(id)
	require.True(t, ok)
	geoStatus := entry.GeoRedundancy.TargetAvailabilityZones[0].Status
	assert.Equal(t, "replicated", geoStatus)
	assert.Equal(t, "passive", entry.Status)

	calls := db.CallCounts()
	assert.Equal(t, 2, calls.Get)
	assert.Equal(t, 1, calls.Patch)
	assert.Equal(t, 1, calls.Resume)
}

func TestStatus(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	id := uuid.New().String()
	db, api := setupMockCG(t, []sgcpcgtesthelper.CgEntry{
		{
			UUID:             id,
			AvailabilityZone: region1AZ1,
			Status:           "ready",
			Replication: sgcpcgtesthelper.ReplicationInfo{
				ReplicationMode: "sync",
				TargetAvailabilityZones: []sgcpcgtesthelper.AZStatus{
					{AvailabilityZone: region1AZ2, Status: "replicated"},
				},
			},
		},
	})
	drv := newTestDriver(t, id, region1AZ1, 5*time.Second, false, api)

	ctx := context.Background()
	statusVal := drv.Status(ctx)
	assert.Equal(t, status.NotApplicable, statusVal)

	calls := db.CallCounts()
	expectedGetCall := 1
	assert.Equal(t, expectedGetCall, calls.Get)
	assert.Equal(t, 0, calls.Patch)

	statusVal = drv.Status(ctx)
	statusVal = drv.Status(ctx)
	statusVal = drv.Status(ctx)
	statusVal = drv.Status(ctx)
	statusVal = drv.Status(ctx)
	calls = db.CallCounts()
	assert.Equal(t, expectedGetCall, calls.Get, "cache has not been used as expected")

	drv.mgr.cacheClearGetCg()
	statusVal = drv.Status(ctx)
	calls = db.CallCounts()
	assert.Equal(t, expectedGetCall+1, calls.Get, "expected call get after clear cache")
}
