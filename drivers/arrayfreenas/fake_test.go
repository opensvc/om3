package arrayfreenas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

// fakeNAS is a TrueNAS api holding datasets, extents, targets, initiators
// and targetextents, with the validations the commands rely on the array
// for: a dataset, an extent name and a targetextent are unique.
//
// The validations not known from the api documentation are marked: the
// fake refuses what the driver must not ask, so a driver asking it fails
// here whatever the real array does.
type fakeNAS struct {
	mu            sync.Mutex
	datasets      []fakeDataset
	extents       []ISCSIExtent
	targets       ISCSITargets
	initiators    ISCSIInitiators
	targetExtents ISCSITargetExtents
	nextID        int

	// requests is every request served, as "METHOD escapedpath?query".
	requests []string

	// fail answers a 500 to the requests whose "METHOD path" begins with a
	// key, the value being the number of such requests to let through
	// first.
	fail map[string]int

	// failAfterWrite makes the array act on the requests whose "METHOD
	// path" begins with a key, then answer a 500, as a timeout does.
	failAfterWrite map[string]bool
}

type fakeDataset struct {
	Id      string          `json:"id"`
	Name    string          `json:"name"`
	Pool    string          `json:"pool"`
	Type    string          `json:"type"`
	Volsize *CompositeValue `json:"volsize,omitempty"`
}

const (
	testHBA1 = "iqn.1993-08.org.debian:01:abcdef"
	testHBA2 = "iqn.1993-08.org.debian:01:123456"
	testTgt1 = "iqn.2005-10.org.freenas.ctl:tgt1"
	testTgt2 = "iqn.2005-10.org.freenas.ctl:tgt2"
)

func newFakeNAS() *fakeNAS {
	f := &fakeNAS{
		nextID:         100,
		fail:           make(map[string]int),
		failAfterWrite: make(map[string]bool),
	}
	f.datasets = []fakeDataset{{Id: "DG", Name: "DG", Pool: "DG", Type: DatasetTypeFilesystem}}
	f.initiators = ISCSIInitiators{
		{Id: 1, Initiators: []string{testHBA1}},
		{Id: 2, Initiators: []string{testHBA2}},
	}
	groups := ISCSITargetGroups{
		{PortalId: 1, InitiatorId: 1, AuthMethod: "NONE"},
		{PortalId: 1, InitiatorId: 2, AuthMethod: "NONE"},
	}
	f.targets = ISCSITargets{
		{Id: 1, Name: testTgt1, Mode: "ISCSI", Groups: groups},
		{Id: 2, Name: testTgt2, Mode: "ISCSI", Groups: groups},
	}
	return f
}

func (f *fakeNAS) id() int {
	f.nextID++
	return f.nextID
}

func (f *fakeNAS) addZvol(name string, size int64) {
	v := CompositeValue{Rawvalue: fmt.Sprint(size)}
	f.datasets = append(f.datasets, fakeDataset{Id: name, Name: name, Pool: strings.Split(name, "/")[0], Type: DatasetTypeVolume, Volsize: &v})
}

func (f *fakeNAS) addExtent(name, zvol string) ISCSIExtent {
	id := f.id()
	e := ISCSIExtent{
		Id:   id,
		Name: name,
		Type: "DISK",
		Path: "zvol/" + zvol,
		Disk: "zvol/" + zvol,
		NAA:  fmt.Sprintf("0x6589cfc000000%019d", id),
	}
	f.extents = append(f.extents, e)
	return e
}

func (f *fakeNAS) addTargetExtent(target, extent, lun int) ISCSITargetExtent {
	te := ISCSITargetExtent{Id: f.id(), TargetId: target, ExtentId: extent, LunId: lun}
	f.targetExtents = append(f.targetExtents, te)
	return te
}

func (f *fakeNAS) dataset(name string) *fakeDataset {
	for i := range f.datasets {
		if f.datasets[i].Name == name {
			return &f.datasets[i]
		}
	}
	return nil
}

// writes returns the requests that change the array.
func (f *fakeNAS) writes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	l := make([]string, 0)
	for _, s := range f.requests {
		if !strings.HasPrefix(s, http.MethodGet) {
			l = append(l, s)
		}
	}
	return l
}

func (f *fakeNAS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.EscapedPath(), Head)
	line := r.Method + " " + path
	if r.URL.RawQuery != "" {
		f.requests = append(f.requests, line+"?"+r.URL.RawQuery)
	} else {
		f.requests = append(f.requests, line)
	}
	reply := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	for prefix, n := range f.fail {
		if strings.HasPrefix(line, prefix) {
			if n == 0 {
				reply(http.StatusInternalServerError, map[string]string{"message": "injected failure"})
				return
			}
			f.fail[prefix] = n - 1
		}
	}
	afterWrite := false
	for prefix := range f.failAfterWrite {
		if strings.HasPrefix(line, prefix) {
			afterWrite = true
		}
	}
	ok := func(v any) {
		if afterWrite {
			reply(http.StatusInternalServerError, map[string]string{"message": "injected failure after write"})
			return
		}
		reply(http.StatusOK, v)
	}
	// A listing not told limit=0 is paged by the api: answering the
	// first page only is answering an empty one.
	listing := func(v any) {
		if r.URL.Query().Get("limit") != "0" {
			reply(http.StatusOK, []any{})
			return
		}
		reply(http.StatusOK, v)
	}
	idOf := func(prefix string) (string, bool) {
		s, ok := strings.CutPrefix(path, prefix)
		if !ok || s == "" || strings.Contains(s, "/") {
			return "", false
		}
		s, err := url.PathUnescape(s)
		return s, err == nil
	}
	intIdOf := func(prefix string) (int, bool) {
		s, ok := idOf(prefix)
		if !ok {
			return 0, false
		}
		i, err := strconv.Atoi(s)
		return i, err == nil
	}

	switch {
	case r.Method == http.MethodGet && path == "/pool/dataset":
		name := r.URL.Query().Get("name")
		l := make([]fakeDataset, 0)
		for _, d := range f.datasets {
			if name == "" || d.Name == name {
				l = append(l, d)
			}
		}
		listing(l)
	case r.Method == http.MethodPost && path == "/pool/dataset":
		var params CreateDatasetParams
		_ = json.Unmarshal(body, &params)
		if f.dataset(params.Name) != nil {
			reply(422, map[string][]string{"pool_dataset_create.name": {"Path " + params.Name + " already exists"}})
			return
		}
		if params.Type == nil || *params.Type != DatasetTypeVolume || params.Volsize == nil {
			reply(422, map[string][]string{"pool_dataset_create.volsize": {"required for a VOLUME"}})
			return
		}
		f.addZvol(params.Name, *params.Volsize)
		ok(f.datasets[len(f.datasets)-1])
	case strings.HasPrefix(path, "/pool/dataset/id/"):
		id, found := idOf("/pool/dataset/id/")
		if !found {
			reply(404, map[string]string{"message": "no such route " + path})
			return
		}
		for i, d := range f.datasets {
			if d.Id != id {
				continue
			}
			switch r.Method {
			case http.MethodDelete:
				// Assumed: the array is not asked to delete a zvol an
				// extent exports.
				for _, e := range f.extents {
					if e.Disk == "zvol/"+d.Name {
						reply(422, map[string][]string{"pool_dataset_delete.id": {"in use by extent " + e.Name}})
						return
					}
				}
				f.datasets = append(f.datasets[:i], f.datasets[i+1:]...)
				ok(true)
			case http.MethodPut:
				var params UpdateDatasetParams
				_ = json.Unmarshal(body, &params)
				if params.Volsize != nil {
					f.datasets[i].Volsize = &CompositeValue{Rawvalue: fmt.Sprint(*params.Volsize)}
				}
				ok(f.datasets[i])
			default:
				reply(405, nil)
			}
			return
		}
		reply(404, map[string]string{"message": "dataset " + id + " not found"})
	case r.Method == http.MethodGet && path == "/iscsi/extent":
		listing(f.extents)
	case r.Method == http.MethodPost && path == "/iscsi/extent":
		var params CreateISCSIExtentParams
		_ = json.Unmarshal(body, &params)
		for _, e := range f.extents {
			if e.Name == params.Name {
				reply(422, map[string][]string{"iscsi_extent_create.name": {"Extent name must be unique"}})
				return
			}
		}
		if f.dataset(strings.TrimPrefix(params.Disk, "zvol/")) == nil {
			reply(422, map[string][]string{"iscsi_extent_create.disk": {"no such zvol"}})
			return
		}
		ok(f.addExtent(params.Name, strings.TrimPrefix(params.Disk, "zvol/")))
	case strings.HasPrefix(path, "/iscsi/extent/id/") && r.Method == http.MethodDelete:
		id, found := intIdOf("/iscsi/extent/id/")
		if !found {
			reply(404, nil)
			return
		}
		// Assumed: the array is not asked to delete an extent still
		// attached to a target.
		for _, te := range f.targetExtents {
			if te.ExtentId == id {
				reply(422, map[string][]string{"iscsi_extent_delete.id": {"attached to target"}})
				return
			}
		}
		for i, e := range f.extents {
			if e.Id == id {
				f.extents = append(f.extents[:i], f.extents[i+1:]...)
				ok(true)
				return
			}
		}
		reply(404, nil)
	case r.Method == http.MethodGet && path == "/iscsi/target":
		listing(f.targets)
	case r.Method == http.MethodGet && path == "/iscsi/initiator":
		listing(f.initiators)
	case r.Method == http.MethodGet && path == "/iscsi/targetextent":
		listing(f.targetExtents)
	case r.Method == http.MethodPost && path == "/iscsi/targetextent":
		var params CreateISCSITargetExtentParams
		_ = json.Unmarshal(body, &params)
		lun := 0
		for _, te := range f.targetExtents {
			if te.TargetId != params.Target {
				continue
			}
			if te.ExtentId == params.Extent {
				reply(422, map[string][]string{"iscsi_targetextent_create.extent": {"Extent is already in this target."}})
				return
			}
			if params.LunId != nil && te.LunId == *params.LunId {
				reply(422, map[string][]string{"iscsi_targetextent_create.lunid": {"LUN ID is already being used for this target."}})
				return
			}
			if te.LunId >= lun {
				lun = te.LunId + 1
			}
		}
		if params.LunId != nil {
			lun = *params.LunId
		}
		ok(f.addTargetExtent(params.Target, params.Extent, lun))
	case strings.HasPrefix(path, "/iscsi/targetextent/id/") && r.Method == http.MethodDelete:
		id, found := intIdOf("/iscsi/targetextent/id/")
		if !found {
			reply(404, nil)
			return
		}
		for i, te := range f.targetExtents {
			if te.Id == id {
				f.targetExtents = append(f.targetExtents[:i], f.targetExtents[i+1:]...)
				ok(true)
				return
			}
		}
		reply(404, nil)
	default:
		reply(404, map[string]string{"message": "unhandled " + line})
	}
}

// newFakeArray returns an array driver configured to talk to f.
func newFakeArray(t *testing.T, f *fakeNAS) *Array {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	// The fake listens on the loopback, which the ssrf policy blocks.
	allowedURL, blockedURL := rawconfig.SSRFAllowedURL, rawconfig.SSRFBlockedURL
	allowedCIDR, blockedCIDR := rawconfig.SSRFAllowedCIDR, rawconfig.SSRFBlockedCIDR
	t.Cleanup(func() {
		rawconfig.SSRFAllowedURL, rawconfig.SSRFBlockedURL = allowedURL, blockedURL
		rawconfig.SSRFAllowedCIDR, rawconfig.SSRFBlockedCIDR = allowedCIDR, blockedCIDR
	})
	rawconfig.SSRFAllowedURL = []string{srv.URL + "/*"}
	rawconfig.SSRFBlockedURL = []string{}
	rawconfig.SSRFAllowedCIDR = []string{"127.0.0.0/8"}
	rawconfig.SSRFBlockedCIDR = []string{}

	config := fmt.Sprintf(`
[array#A]
type = freenas
api = %s
username = root
password = system/sec/nas
`, srv.URL)
	n, err := object.NewNode(object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)
	a := New()
	a.SetName("array#A")
	a.SetConfig(n.MergedConfig())
	a.secret = func() (string, error) { return "secret", nil }
	return a
}

// run runs a command line through the actions of a, as the collector's
// queued "om node array" does, and returns what it printed.
func run(t *testing.T, a *Array, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := array.RunActions(context.Background(), a.Actions(), args, &buf)
	return buf.String(), err
}
