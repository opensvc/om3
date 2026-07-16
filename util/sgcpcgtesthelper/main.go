package sgcpcgtesthelper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type AZStatus struct {
	AvailabilityZone string `json:"availabilityZone"`
	Status           string `json:"status"`
}

type GeoRedundancyInfo struct {
	Region                  string     `json:"region"`
	TargetAvailabilityZones []AZStatus `json:"targetAvailabilityZones"`
}

type ReplicationInfo struct {
	ReplicationMode         string     `json:"replicationMode"`
	TargetAvailabilityZones []AZStatus `json:"targetAvailabilityZones"`
}

type CgEntry struct {
	UUID             string            `json:"uuid"`
	Name             string            `json:"name"`
	AvailabilityZone string            `json:"availabilityZone"`
	Status           string            `json:"status"`
	GeoRedundancy    GeoRedundancyInfo `json:"georedundancy"`
	Replication      ReplicationInfo   `json:"replication"`
}

type DB struct {
	mu                      sync.RWMutex
	byUUID                  map[string]*CgEntry
	callCount               Counters
	PatchSwitchoverFunc     func(ctx context.Context, uuid, targetAZ string) error
	PatchFailoverFunc       func(ctx context.Context, uuid, targetAZ string) error
	PatchResumeFunc         func(ctx context.Context, uuid string) error
	GetConsistencyGroupFunc func(ctx context.Context, uuid string) (method, url string, code int, data []byte, err error)
}

type Counters struct {
	Get    int
	Patch  int
	Switch int
	Fail   int
	Resume int
}

type API struct {
	*DB
}

func NewDB() *DB {
	return &DB{
		byUUID: make(map[string]*CgEntry),
	}
}

func NewAPI(db *DB) *API {
	return &API{DB: db}
}

func (db *DB) Setup(entries []CgEntry) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.byUUID = make(map[string]*CgEntry)
	for _, e := range entries {
		db.byUUID[e.UUID] = deepCopy(&e)
	}
}

func (db *DB) ResetCalls() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.callCount = Counters{}
}

func (db *DB) CallCounts() Counters {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.callCount
}

func (a *API) GetConsistencyGroup(ctx context.Context, uuid string) (method, url string, code int, data []byte, err error) {
	_ = ctx
	if a.GetConsistencyGroupFunc != nil {
		return a.GetConsistencyGroupFunc(ctx, uuid)
	}
	a.mu.Lock()
	a.callCount.Get++
	a.mu.Unlock()

	a.mu.RLock()
	defer a.mu.RUnlock()
	entry, ok := a.byUUID[uuid]
	if !ok {
		return http.MethodGet, "/consistency-groups/" + uuid, http.StatusNotFound, nil, nil
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return http.MethodGet, "/consistency-groups/" + uuid, http.StatusInternalServerError, nil, err
	}
	return http.MethodGet, "/consistency-groups/" + uuid, http.StatusOK, b, nil
}

func (a *API) PatchConsistencyGroup(ctx context.Context, uuid string, payload any) (method, url string, code int, data []byte, err error) {
	a.mu.Lock()
	a.callCount.Patch++
	a.mu.Unlock()

	a.mu.RLock()
	entry, ok := a.byUUID[uuid]
	a.mu.RUnlock()
	if !ok {
		return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusNotFound, nil, fmt.Errorf("cg not found")
	}

	payloadMap, ok := payload.(map[string]any)
	if !ok {
		return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusBadRequest, nil, fmt.Errorf("invalid payload type")
	}
	op, _ := payloadMap["operation"].(string)

	switch op {
	case "switchover":
		a.mu.Lock()
		a.callCount.Switch++
		a.mu.Unlock()
		if a.PatchSwitchoverFunc != nil {
			params, _ := payloadMap["operationParameters"].(map[string]any)
			targetAZ, _ := params["availabilityZone"].(string)
			if err := a.PatchSwitchoverFunc(ctx, uuid, targetAZ); err != nil {
				return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusPreconditionFailed, nil, err
			}
		}
		a.mu.Lock()
		params, _ := payloadMap["operationParameters"].(map[string]any)
		if targetAZ, ok := params["availabilityZone"].(string); ok {
			entry.AvailabilityZone = targetAZ
			entry.Status = "ready"
		}
		a.mu.Unlock()
		return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusAccepted, nil, nil

	case "failover":
		a.mu.Lock()
		a.callCount.Fail++
		a.mu.Unlock()
		if a.PatchFailoverFunc != nil {
			params, _ := payloadMap["operationParameters"].(map[string]any)
			targetAZ, _ := params["availabilityZone"].(string)
			if err := a.PatchFailoverFunc(ctx, uuid, targetAZ); err != nil {
				return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusInternalServerError, nil, err
			}
		}
		a.mu.Lock()
		params, _ := payloadMap["operationParameters"].(map[string]any)
		if targetAZ, ok := params["availabilityZone"].(string); ok {
			entry.AvailabilityZone = targetAZ
			entry.Status = "ready"
		}
		a.mu.Unlock()
		return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusAccepted, nil, nil

	case "resume-replication":
		a.mu.Lock()
		a.callCount.Resume++
		a.mu.Unlock()
		if a.PatchResumeFunc != nil {
			if err := a.PatchResumeFunc(ctx, uuid); err != nil {
				return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusInternalServerError, nil, err
			}
		}
		a.mu.Lock()
		for i := range entry.Replication.TargetAvailabilityZones {
			if entry.Replication.TargetAvailabilityZones[i].AvailabilityZone == entry.AvailabilityZone {
				entry.Replication.TargetAvailabilityZones[i].Status = "replicated"
			}
		}
		for i := range entry.GeoRedundancy.TargetAvailabilityZones {
			if entry.GeoRedundancy.TargetAvailabilityZones[i].AvailabilityZone == entry.AvailabilityZone {
				entry.GeoRedundancy.TargetAvailabilityZones[i].Status = "replicated"
			}
		}
		entry.Status = "ready"
		a.mu.Unlock()
		return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusAccepted, nil, nil

	default:
		return http.MethodPatch, "/consistency-groups/" + uuid, http.StatusBadRequest, nil, fmt.Errorf("unknown operation %s", op)
	}
}

func (db *DB) Search(uuid string) (*CgEntry, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	entry, ok := db.byUUID[uuid]
	if !ok {
		return nil, false
	}
	return deepCopy(entry), true
}

func (db *DB) Update(entry *CgEntry) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, ok := db.byUUID[entry.UUID]; !ok {
		return fmt.Errorf("entry not found")
	}
	db.byUUID[entry.UUID] = deepCopy(entry)
	return nil
}

func deepCopy(src *CgEntry) *CgEntry {
	if src == nil {
		return nil
	}
	cp := &CgEntry{
		UUID:             src.UUID,
		Name:             src.Name,
		AvailabilityZone: src.AvailabilityZone,
		Status:           src.Status,
	}
	if len(src.GeoRedundancy.TargetAvailabilityZones) > 0 || src.GeoRedundancy.Region != "" {
		cp.GeoRedundancy.Region = src.GeoRedundancy.Region
		cp.GeoRedundancy.TargetAvailabilityZones = make([]AZStatus, len(src.GeoRedundancy.TargetAvailabilityZones))
		for i, az := range src.GeoRedundancy.TargetAvailabilityZones {
			cp.GeoRedundancy.TargetAvailabilityZones[i] = AZStatus{
				AvailabilityZone: az.AvailabilityZone,
				Status:           az.Status,
			}
		}
	}
	if len(src.Replication.TargetAvailabilityZones) > 0 || src.Replication.ReplicationMode != "" {
		cp.Replication.ReplicationMode = src.Replication.ReplicationMode
		cp.Replication.TargetAvailabilityZones = make([]AZStatus, len(src.Replication.TargetAvailabilityZones))
		for i, az := range src.Replication.TargetAvailabilityZones {
			cp.Replication.TargetAvailabilityZones[i] = AZStatus{
				AvailabilityZone: az.AvailabilityZone,
				Status:           az.Status,
			}
		}
	}
	return cp
}
