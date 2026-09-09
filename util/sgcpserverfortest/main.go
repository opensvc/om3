package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/google/uuid"
)

type (
	NfsClient struct {
		UUID               string `json:"uuid"`
		Host               string `json:"host"`
		Permission         string `json:"permission"`
		Protocol           string `json:"protocol"`
		ConsistencyGroupID string `json:"consistencyGroupId,omitempty"`
	}

	FilesystemInfo struct {
		UUID               string      `json:"uuid"`
		ConsistencyGroupID string      `json:"consistencyGroupId"`
		NFSClients         []NfsClient `json:"nfsClients"`
		Status             string      `json:"status"`
	}

	Token struct {
		AccessToken string `json:"access_token"`
	}

	User struct {
		ClientID     string   `yaml:"client_id"`
		ClientSecret string   `yaml:"client_secret"`
		Scopes       []string `yaml:"scopes"`
	}

	Users map[string]User

	Alias struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Target string `json:"target"`
		FQDN   string `json:"fqdn"`
		ZoneID string `json:"zoneId"`
	}

	CgInfo struct {
		UUID             string         `json:"uuid"`
		Name             string         `json:"name"`
		AvailabilityZone string         `json:"availabilityZone"`
		Status           string         `json:"status"`
		GeoRedundancy    map[string]any `json:"georedundancy"`
		Replication      map[string]any `json:"replication"`
	}
)

var (
	users *Users

	files = map[string]FilesystemInfo{
		"1ab7d139-dd35-4f9c-ad82-cd6a93675cfd": {
			UUID:               "1ab7d139-dd35-4f9c-ad82-cd6a93675cfd",
			ConsistencyGroupID: "12",
			NFSClients:         []NfsClient{},
			Status:             "online",
		},
	}

	createdTokenCount atomic.Int64

	aliasMu    sync.Mutex
	aliasStore = map[string]Alias{}

	cgStore = map[string]*CgInfo{
		"1ab7d139-dd35-4f9c-ad82-cd6a93675cfd": {
			UUID:             "1ab7d139-dd35-4f9c-ad82-cd6a93675cfd",
			Name:             "test-cg",
			AvailabilityZone: "az2",
			Status:           "passive",
			GeoRedundancy:    map[string]any{},
			Replication:      map[string]any{},
		},
	}
	cgMu sync.Mutex

	permissiveAuth = false
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("HTTP request", "method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery)
		next.ServeHTTP(w, r)
	})
}

func assertAuth(desc string, w http.ResponseWriter, r *http.Request, scope ...string) bool {
	if permissiveAuth {
		return true
	}
	auth := r.Header.Get("Authorization")
	for _, s := range scope {
		if !strings.Contains(auth, s) {
			slog.Warn(fmt.Sprintf("%s [%d]", desc, http.StatusForbidden), "missing_scope", s)
			w.WriteHeader(http.StatusForbidden)
			return false
		}
	}
	return true
}

func logStatusCode(desc string, code int) {
	slog.Info(fmt.Sprintf("%s [%d]", desc, code), "code", code)
}

func setHeader(w http.ResponseWriter, desc string, code int) {
	logStatusCode(desc, code)
	w.WriteHeader(code)
}

// ========== File handlers ==========
func getFileAPI(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slog.Info(GetFile, "id", id)
	if !assertAuth(GetFile, w, r, "account1:sgcp:files:read") {
		return
	}
	if v, ok := files[id]; ok {
		setHeader(w, GetFile, http.StatusOK)
		json.NewEncoder(w).Encode(v)
		return
	}
	setHeader(w, GetFile, http.StatusNotFound)
}

func getFileClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slog.Info(GetFileClient, "id", id)
	if !assertAuth(GetFileClient, w, r, "account1:sgcp:files:read") {
		return
	}
	if v, ok := files[id]; ok {
		setHeader(w, GetFileClient, http.StatusOK)
		json.NewEncoder(w).Encode(v.NFSClients)
		return
	}
	setHeader(w, GetFileClient, http.StatusNotFound)
}

func postFileClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slog.Info(PostFileClient, "id", id)
	if !assertAuth(PostFileClient, w, r, "account1:sgcp:files:read", "account1:sgcp:files:write") {
		return
	}
	var body NfsClient
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logStatusCode(PostFileClient, http.StatusBadRequest)
		http.Error(w, fmt.Sprintf("invalid JSON: %s", err), http.StatusBadRequest)
		return
	}
	if v, ok := files[id]; ok {
		body.UUID = uuid.New().String()
		v.NFSClients = append(v.NFSClients, body)
		files[id] = v
		setHeader(w, PostFileClient, http.StatusCreated)
		json.NewEncoder(w).Encode(body)
		slog.Info("Created client", "id", id, "clientID", body.UUID)
		return
	}
	logStatusCode(PostFileClient, http.StatusNotFound)
	http.Error(w, fmt.Sprintf("no such fs %s", id), http.StatusNotFound)
}

func deleteFileClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	clientID := r.PathValue("clientID")
	slog.Info(DeleteFileClient, "id", id, "clientID", clientID)
	if !assertAuth(DeleteFileClient, w, r, "account1:sgcp:files:read", "account1:sgcp:files:write") {
		return
	}
	if v, ok := files[id]; ok {
		l := make([]NfsClient, 0)
		for _, c := range v.NFSClients {
			if c.UUID != clientID {
				l = append(l, c)
			}
		}
		v.NFSClients = l
		files[id] = v
		setHeader(w, DeleteFileClient, http.StatusNoContent)
		return
	}
	logStatusCode(DeleteFileClient, http.StatusNotFound)
	http.Error(w, fmt.Sprintf("no such fs %s", id), http.StatusNotFound)
}

// ========== DNS handlers ==========
func dnsListAliases(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	name := r.URL.Query().Get("name")
	id := r.URL.Query().Get("id")

	slog.Info(ListDnsAliases, "zoneID", zoneID, "name", name, "id", id)
	if !assertAuth(ListDnsAliases, w, r, "account1:sgcp:dns:read") {
		return
	}

	aliasMu.Lock()
	defer aliasMu.Unlock()
	var out []Alias
	for _, a := range aliasStore {
		if a.ZoneID != zoneID {
			continue
		}
		if name != "" && a.Name != name {
			continue
		}
		if id != "" && a.ID != id {
			continue
		}
		out = append(out, a)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"cnameRecords": out})
}

func dnsCreateAlias(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	slog.Info(CreateDnsAlias, "zoneID", zoneID)
	if !assertAuth(CreateDnsAlias, w, r, "account1:sgcp:dns:read", "account1:sgcp:dns:write") {
		return
	}
	var payload struct {
		Name   string `json:"name"`
		Target string `json:"target"`
		TTL    int    `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Info("POST /dns/zones/{zoneID}/cname-records invalid JSON", "err", err)
		http.Error(w, fmt.Sprintf("invalid JSON: %s", err), http.StatusBadRequest)
		return
	}
	slog.Info("POST /dns/zones/{zoneID}/cname-records", "name", payload.Name, "target", payload.Target)

	id := uuid.New().String()
	a := Alias{
		ID:     id,
		Name:   payload.Name,
		Target: payload.Target,
		FQDN:   fmt.Sprintf("%s.%s", payload.Name, zoneID),
		ZoneID: zoneID,
	}
	aliasMu.Lock()
	aliasStore[id] = a
	aliasMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
}

func dnsGetAlias(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	id := r.PathValue("id")
	slog.Info(GetDnsAlias, "zoneID", zoneID, "id", id)
	if !assertAuth(GetDnsAlias, w, r, "account1:sgcp:dns:read") {
		return
	}
	aliasMu.Lock()
	defer aliasMu.Unlock()
	a, ok := aliasStore[id]
	if !ok || a.ZoneID != zoneID {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(a)
}

func dnsUpdateAlias(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	id := r.PathValue("id")
	slog.Info(UpdateDnsAlias, "zoneID", zoneID, "id", id)
	if !assertAuth(UpdateDnsAlias, w, r, "account1:sgcp:dns:read", "account1:sgcp:dns:write") {
		return
	}
	var payload struct {
		Target string `json:"target"`
		TTL    int    `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %s", err), http.StatusBadRequest)
		return
	}
	aliasMu.Lock()
	defer aliasMu.Unlock()
	a, ok := aliasStore[id]
	if !ok || a.ZoneID != zoneID {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if payload.Target != "" {
		a.Target = payload.Target
	}
	aliasStore[id] = a
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(a)
}

func dnsDeleteAlias(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	id := r.PathValue("id")
	slog.Info(DeleteDnsAlias, "zoneID", zoneID, "id", id)
	if !assertAuth(DeleteDnsAlias, w, r, "account1:sgcp:dns:read", "account1:sgcp:dns:write") {
		return
	}
	aliasMu.Lock()
	defer aliasMu.Unlock()
	a, ok := aliasStore[id]
	if !ok || a.ZoneID != zoneID {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	delete(aliasStore, id)
	w.WriteHeader(http.StatusNoContent)
}

// ========== CG handlers ==========
func getCG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slog.Info(GetCG, "id", id)
	if !assertAuth(GetCG, w, r, "account1:sgcp:files:read") {
		return
	}
	cgMu.Lock()
	defer cgMu.Unlock()
	cg, ok := cgStore[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	setHeader(w, GetCG, http.StatusOK)
	json.NewEncoder(w).Encode(cg)
}

func patchCG(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slog.Info(PatchCG, "id", id)
	if !assertAuth(PatchCG, w, r, "account1:sgcp:files:read", "account1:sgcp:files:write") {
		return
	}
	cgMu.Lock()
	defer cgMu.Unlock()
	cg, ok := cgStore[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var payload struct {
		Operation           string         `json:"operation"`
		OperationParameters map[string]any `json:"operationParameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	slog.Info("PATCH /file/cg/{id} operation", "operation", payload.Operation)
	switch payload.Operation {
	case "switchover":
		if az, ok := payload.OperationParameters["availabilityZone"].(string); ok {
			cg.AvailabilityZone = az
		}
		cg.Status = "ready"
	case "failover":
		if az, ok := payload.OperationParameters["availabilityZone"].(string); ok {
			cg.AvailabilityZone = az
		}
		cg.Status = "ready"
	case "resume-replication":
		cg.Status = "ready"
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// ========== Auth ==========
func postAuthToken(w http.ResponseWriter, r *http.Request) {
	slog.Info(PostAuth)
	if permissiveAuth {
		requestedScopes := []string{
			"account1:sgcp:files:read",
			"account1:sgcp:files:write",
			"account1:sgcp:dns:read",
			"account1:sgcp:dns:write",
		}
		count := createdTokenCount.Add(1)
		body := Token{AccessToken: fmt.Sprintf("%v", requestedScopes)}
		setHeader(w, PostAuth, http.StatusOK)
		slog.Info(PostAuth, "createdCount", count, "createdToken", requestedScopes)
		json.NewEncoder(w).Encode(body)
		return
	}

	auth := r.Header.Get("Authorization")
	authB, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		http.Error(w, "invalid auth", http.StatusBadRequest)
		return
	}
	l := strings.Split(string(authB), ":")
	if len(l) != 2 {
		http.Error(w, "invalid auth", http.StatusBadRequest)
		return
	}
	clientID := l[0]
	clientSecret := l[1]
	userScopes, ok := users.ScopesForAuth(clientID, clientSecret)
	if !ok {
		slog.Info(PostAuth+" bad credentials", "client_id", clientID)
		setHeader(w, PostAuth, http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	formScope := r.FormValue("scope")
	requestedScopes := strings.Fields(formScope)
	for _, s := range requestedScopes {
		if !slices.Contains(userScopes, s) {
			slog.Warn(PostAuth, "refusedScope", s)
			setHeader(w, PostAuth, http.StatusForbidden)
			return
		}
	}
	count := createdTokenCount.Add(1)
	body := Token{AccessToken: fmt.Sprintf("%v", requestedScopes)}
	setHeader(w, PostAuth, http.StatusOK)
	slog.Info(PostAuth, "createdCount", count, "client_id", clientID, "createdToken", requestedScopes)
	json.NewEncoder(w).Encode(body)
}

func (u Users) ScopesForAuth(clientID, clientSecret string) ([]string, bool) {
	if user, ok := u[clientID]; ok && user.ClientSecret == clientSecret {
		return user.Scopes, true
	}
	return nil, false
}

// ========== Routes ==========
var (
	GetFile          = "GET /file/fs/{id}"
	PostFileClient   = "POST /file/fs/{id}/client"
	GetFileClient    = "GET /file/fs/{id}/client"
	DeleteFileClient = "DELETE /file/fs/{id}/client/{clientID}"

	ListDnsAliases = "GET /dns/zones/{zoneID}/cname-records"
	CreateDnsAlias = "POST /dns/zones/{zoneID}/cname-records"
	GetDnsAlias    = "GET /dns/zones/{zoneID}/cname-records/{id}"
	UpdateDnsAlias = "PATCH /dns/zones/{zoneID}/cname-records/{id}"
	DeleteDnsAlias = "DELETE /dns/zones/{zoneID}/cname-records/{id}"

	GetCG   = "GET /file/cg/{id}"
	PatchCG = "PATCH /file/cg/{id}"

	PostAuth = "POST /auth/access_token"
)

func loadUsers() (*Users, error) {
	var u Users
	configFile := "users.yaml"
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}
	if err := yaml.Unmarshal(data, &u); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}
	for userID, user := range u {
		slog.Info("loaded user", "userID", userID, "clientID", user.ClientID, "scopes", user.Scopes)
	}
	return &u, nil
}

func main() {
	var err error
	createdTokenCount.Store(0)

	if os.Getenv("SGCP_TEST_AUTH_DISABLED") == "1" {
		permissiveAuth = true
		slog.Info("Permissive authentication enabled")
	}

	users, err = loadUsers()
	if err != nil {
		slog.Error("failed to load users", "err", err)
		return
	}

	mux := http.NewServeMux()

	// File
	mux.HandleFunc(GetFile, getFileAPI)
	mux.HandleFunc(PostFileClient, postFileClient)
	mux.HandleFunc(GetFileClient, getFileClient)
	mux.HandleFunc(DeleteFileClient, deleteFileClient)

	// DNS
	mux.HandleFunc(ListDnsAliases, dnsListAliases)
	mux.HandleFunc(CreateDnsAlias, dnsCreateAlias)
	mux.HandleFunc(GetDnsAlias, dnsGetAlias)
	mux.HandleFunc(UpdateDnsAlias, dnsUpdateAlias)
	mux.HandleFunc(DeleteDnsAlias, dnsDeleteAlias)

	// CG
	mux.HandleFunc(GetCG, getCG)
	mux.HandleFunc(PatchCG, patchCG)

	// Auth
	mux.HandleFunc(PostAuth, postAuthToken)

	// Default
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Unmatched route", "method", r.Method, "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})

	handler := loggingMiddleware(mux)

	log.Println("Listening on :8000")
	log.Fatal(http.ListenAndServe(":8000", handler))
}
