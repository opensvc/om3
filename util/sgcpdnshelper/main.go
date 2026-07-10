package sgcpdnshelper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/util/sgcp"
)

type (
	Api struct {
		sgcp.Api
		*DB
	}

	DB struct {
		rLock           sync.RWMutex
		byId            map[string]*DBEntry
		byZoneAndName   map[string]*DBEntry
		byIdZoneAndName map[string]*DBEntry

		callCount Counters
	}

	Counters struct {
		Delete int
		Update int
		Search int
	}

	DBEntry struct {
		Name       string
		UUID       string
		ZoneID     string
		AliasL     []sgcp.Alias
		StatusCode int
		Err        error
	}
)

func NewDB() *DB {
	return &DB{
		byId:            make(map[string]*DBEntry),
		byZoneAndName:   make(map[string]*DBEntry),
		byIdZoneAndName: make(map[string]*DBEntry),
	}
}

func NewApi(db *DB) *Api {
	return &Api{DB: db}
}

func (a *Api) CreateAlias(_ context.Context, zoneID, name, target string) (*sgcp.Alias, error) {
	aUUID := uuid.New().String()
	alias := sgcp.Alias{
		UUID:   aUUID,
		Name:   name,
		Target: target,
		FQDN:   fmt.Sprintf("%s.%s", name, zoneID),
		ZoneID: zoneID,
	}
	v := &DBEntry{
		Name:   name,
		UUID:   aUUID,
		ZoneID: zoneID,
		AliasL: []sgcp.Alias{alias},
	}
	a.update(v)
	return &v.AliasL[0], nil
}

func (a *Api) DeleteAlias(_ context.Context, zoneID, aliasUUID string) error {
	v, ok := a.search(zoneID, "", aliasUUID)
	if ok {
		a.delete(v)
	}
	return nil
}

func (a *Api) GetAliases(_ context.Context, zoneID, name, uuid string) (method, url string, code int, data []byte, err error) {
	resp, ok := a.search(zoneID, name, uuid)
	a.rLock.RLock()
	a.callCount.Search++
	a.rLock.RUnlock()
	method = http.MethodGet
	url = "/file/alias"
	if ok {
		code := resp.StatusCode
		if code == 0 {
			code = http.StatusOK
		}
		m := map[string][]sgcp.Alias{
			"aliases": resp.AliasL,
		}
		b, err := json.Marshal(m)
		if err == nil {
			err = resp.Err
		}
		return method, url, code, b, err
	}
	return method, url, http.StatusNotFound, nil, nil
}

func (t *Api) UpdateAlias(_ context.Context, zoneID string, aliasUUID string, name string, target string) (*sgcp.Alias, error) {
	v, ok := t.search(zoneID, name, aliasUUID)
	if !ok {
		return nil, fmt.Errorf("alias not found")
	}
	if len(v.AliasL) != 1 {
		return nil, fmt.Errorf("can't update: len aliases=%d", len(v.AliasL))
	}
	v.AliasL[0].Target = target
	t.update(v)
	return v.asAlias(), nil
}

func (a *DB) delete(v *DBEntry) {
	a.rLock.Lock()
	defer a.rLock.Unlock()
	delete(a.byId, v.UUID)
	delete(a.byZoneAndName, v.Name+"@"+v.ZoneID)
	delete(a.byIdZoneAndName, v.Name+"@"+v.ZoneID+"@"+v.UUID)
	a.callCount.Delete++
}

func (a *DB) update(v *DBEntry) {
	if v == nil {
		return
	}
	nv := v.clone()
	a.rLock.Lock()
	defer a.rLock.Unlock()
	if nv.UUID != "" {
		a.byId[nv.UUID] = nv
	}
	if nv.Name != "" && nv.ZoneID != "" {
		a.byZoneAndName[nv.Name+"@"+nv.ZoneID] = nv
	}
	if nv.UUID != "" && nv.Name != "" && nv.ZoneID != "" {
		a.byIdZoneAndName[nv.Name+"@"+nv.ZoneID+"@"+nv.UUID] = nv
	}
	a.callCount.Update++
}

func (a *DB) Setup(l []DBEntry) {
	a.byId = make(map[string]*DBEntry)
	a.byZoneAndName = make(map[string]*DBEntry)
	a.byIdZoneAndName = make(map[string]*DBEntry)
	for _, v := range l {
		a.update(v.clone())
	}
}

func (a *DB) CallCounts() Counters {
	a.rLock.RLock()
	calls := a.callCount
	a.rLock.RUnlock()
	return calls
}

func (a *DB) ResetCalls() {
	a.rLock.Lock()
	defer a.rLock.Unlock()
	a.callCount = Counters{}
}

// search looks up a DBEntry in the database matching the specified zoneID, name, and uuid.
// Returns the matched DBEntry and a boolean indicating success or failure.
func (a *DB) search(zoneID, name, uuid string) (v *DBEntry, ok bool) {
	a.rLock.RLock()
	defer a.rLock.RUnlock()
	if zoneID != "" && name != "" && uuid != "" {
		v, ok = a.byIdZoneAndName[name+"@"+zoneID+"@"+uuid]
	} else if zoneID != "" && name != "" {
		v, ok = a.byZoneAndName[name+"@"+zoneID]
	} else if uuid != "" {
		v, ok = a.byId[uuid]
	}
	v = v.clone()
	return
}

// Search retrieves an sgcp.Alias from the database using zoneID, name, and uuid as search parameters.
// Returns the matched alias and a boolean indicating whether the alias was found.
func (a *DB) Search(zoneID, name, uuid string) (alias *sgcp.Alias, ok bool) {
	v, ok := a.search(zoneID, name, uuid)
	if !ok {
		return nil, ok
	}
	return v.asAlias(), ok
}

func (v *DBEntry) asAlias() *sgcp.Alias {
	if len(v.AliasL) != 1 {
		return nil
	}
	a := v.AliasL[0]
	return &sgcp.Alias{
		UUID:   a.UUID,
		Name:   a.Name,
		Target: a.Target,
		FQDN:   a.FQDN,
		ZoneID: a.ZoneID,
	}
}

func (v *DBEntry) clone() *DBEntry {
	if v == nil {
		return nil
	}
	n := *v
	l := make([]sgcp.Alias, len(v.AliasL))
	for i, a := range v.AliasL {
		l[i] = a
	}
	n.AliasL = l
	return &n
}
