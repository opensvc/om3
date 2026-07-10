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
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/util/sgcp"
	"github.com/opensvc/om3/v3/util/sgcpdnstesthelper"
)

type (
	// NfsClient represents an NFS client configuration
	NfsClient struct {
		UUID               string `json:"uuid"`
		Host               string `json:"host"`
		Permission         string `json:"permission"`
		Protocol           string `json:"protocol"`
		ConsistencyGroupID string `json:"consistencyGroupId,omitempty"`
	}

	// FilesystemInfo represents the filesystem information from the API
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
)

var (
	users *Users

	files = map[string]FilesystemInfo{
		"1ab7d139-dd35-4f9c-ad82-cd6a93675cfd": FilesystemInfo{
			UUID:               "1ab7d139-dd35-4f9c-ad82-cd6a93675cfd",
			ConsistencyGroupID: "12",
			NFSClients:         nil,
			Status:             "online",
		},
	}

	createdTokenCount atomic.Int64
)

func assertAuth(desc string, w http.ResponseWriter, r *http.Request, scope ...string) bool {
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
	return
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

	return
}
func postFileClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slog.Info(PostFileClient, "id", id)
	if !assertAuth(PostFileClient, w, r, "account1:sgcp:files:read", "account1:sgcp:files:write") {
		return
	}

	var body NfsClient
	err := json.NewDecoder(r.Body).Decode(&body)

	if err != nil {
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
	http.Error(w, fmt.Sprintf("no such fs %s", err), http.StatusNotFound)
	return
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
	return
}

func dnsGetAliasHandler(a *sgcpdnstesthelper.Api) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zoneID := r.PathValue("zone_id")
		id := r.PathValue("id")
		query := r.URL.Query()
		qName := query.Get("name")
		qUUID := query.Get("uuid")

		slog.Info(GetDnsAlias, "zoneID", zoneID, "id", id)
		alias, ok := a.DB.Search(zoneID, qName, qUUID)
		if !ok {
			logStatusCode(GetDnsAlias, http.StatusNotFound)
			http.Error(w, fmt.Sprintf("no such alias %s", id), http.StatusNotFound)
			return
		}
		setHeader(w, GetDnsAlias, http.StatusOK)
		// TODO: verify mapping
		body := map[string]sgcp.Alias{
			"alias": *alias,
		}
		json.NewEncoder(w).Encode(body)
	}
}

func postAuthToken(w http.ResponseWriter, r *http.Request) {
	slog.Info(PostAuth)
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
		slog.Info(PostAuth+" bad creadentials", "client_id", clientID)
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
	body := Token{
		AccessToken: fmt.Sprintf("%v", requestedScopes),
	}
	setHeader(w, PostAuth, http.StatusOK)
	slog.Info(PostAuth, "createdCount", count, "client_id", clientID, "createdToken", requestedScopes)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(body)
}

func (u Users) ScopesForAuth(clientID, clientSecret string) ([]string, bool) {
	if user, ok := u[clientID]; ok && user.ClientSecret == clientSecret {
		return user.Scopes, true
	}
	return nil, false
}

var (
	GetFile          = "GET /file/fs/{id}"
	PostFileClient   = "POST /file/fs/{id}/client"
	GetFileClient    = "Get /file/fs/{id}/client"
	DeleteFileClient = "DELETE /file/fs/{id}/client/{clientID}"

	// TODO: verify path
	GetDnsAlias = "GET /dns/zone/{zone_id}/cname-entry/{id}"

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
		slog.Info("loaded user", "userID", userID, "clientSecret", user.ClientSecret, "scopes", user.Scopes)
	}

	fmt.Printf("loaded users: %#v\n", u)

	return &u, nil
}

func main() {
	var err error
	createdTokenCount.Store(0)
	mux := http.NewServeMux()
	dnsDB := sgcpdnstesthelper.NewDB()
	dnsApi := sgcpdnstesthelper.NewApi(dnsDB)

	users, err = loadUsers()
	if err != nil {
		slog.Error("failed to load users", "err", err)
		return
	}
	// url: /file/fs/{id}
	mux.HandleFunc(GetFile, getFileAPI)

	// url: /file/fs/{id}/client
	mux.HandleFunc(PostFileClient, postFileClient)
	mux.HandleFunc(GetFileClient, getFileClient)

	// url: /file/fs/{id}/client/{clientID}
	mux.HandleFunc(DeleteFileClient, deleteFileClient)

	// url: /dns/alias/{id}
	mux.HandleFunc(GetDnsAlias, dnsGetAliasHandler(dnsApi))

	// url: /auth/access_token
	mux.HandleFunc(PostAuth, postAuthToken)

	log.Println("Listening on :8000")
	log.Fatal(http.ListenAndServe(":8000", mux))
}
