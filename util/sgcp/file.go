package sgcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// FilesAPI provides methods for file-related API operations
	FilesAPI struct {
		Api
		config *Config
	}

	// NfsClient represents an NFS client with associated metadata and permissions for a specific filesystem.
	NfsClient struct {
		UUID               string `json:"uuid,omitempty"`
		Host               string `json:"host"`
		Permission         string `json:"permission"`
		Protocol           string `json:"protocol"`
		ConsistencyGroupID string `json:"consistencyGroupId,omitempty"`
	}
)

// NewFilesAPI creates a new FilesAPI instance
func NewFilesAPI(config *Config, client *http.Client, l *plog.Logger, tk tokenGetter) *FilesAPI {
	return &FilesAPI{
		config: config,
		Api: Api{
			client: client,
			tk:     tk,
			log:    l,
		},
	}
}

func (a *FilesAPI) GetFilesystem(ctx context.Context, uuid string) (method, url string, code int, data []byte, err error) {
	method = http.MethodGet
	url = a.getFilesystemURL(uuid)
	code, data, err = a.do(ctx, method, url, nil, a.GetScopes("files_read")...)
	return
}

func (a *FilesAPI) PostNFSClients(ctx context.Context, uuid string, client *NfsClient) (method, url string, code int, data []byte, err error) {
	var b []byte
	method = http.MethodPost
	url = a.getNFSClientsURL(uuid)

	b, err = json.Marshal(client)
	if err != nil {
		err = fmt.Errorf("failed to marshal NFS client: %w", err)
		return
	}
	a.log.Infof("%s %s data=%s", method, url, string(b))
	code, data, err = a.do(ctx, method, url, bytes.NewReader(b), a.GetScopes("files_write")...)
	return
}

func (a *FilesAPI) DeleteNFSClients(ctx context.Context, fsUUID, clientUUID string) (method, url string, code int, data []byte, err error) {
	method = http.MethodDelete
	url = a.getNFSClientURL(fsUUID, clientUUID)

	a.log.Infof("%s %s", method, url)
	code, data, err = a.do(ctx, method, url, nil, a.GetScopes("files_write")...)
	return
}

func (a *FilesAPI) GetScopes(scopeType string) []string {
	return a.config.GetScopes(scopeType)
}

// getFilesystemURL constructs the URL for a specific filesystem
func (a *FilesAPI) getFilesystemURL(uuid string) string {
	return fmt.Sprintf("%s%s/%s", a.config.Files.BaseURL, a.config.Files.Path.FS, uuid)
}

// getNFSClientsURL constructs the URL for NFS clients of a filesystem
func (a *FilesAPI) getNFSClientsURL(uuid string) string {
	return fmt.Sprintf("%s%s/%s%s",
		a.config.Files.BaseURL,
		a.config.Files.Path.FS,
		uuid,
		a.config.Files.Path.Client)
}

// getNFSClientURL constructs the URL for a specific NFS client
func (a *FilesAPI) getNFSClientURL(uuid, clientUUID string) string {
	return fmt.Sprintf("%s%s/%s%s/%s",
		a.config.Files.BaseURL,
		a.config.Files.Path.FS,
		uuid,
		a.config.Files.Path.Client,
		clientUUID)
}

// do is the function that executes the HTTP request
func (a *FilesAPI) do(ctx context.Context, method, url string, body io.Reader, scopes ...string) (statusCode int, b []byte, err error) {
	return a.Api.do(ctx, method, url, body, scopes...)
}

// GetConsistencyGroupURL constructs the URL for a specific consistency group
func (a *FilesAPI) GetConsistencyGroupURL(uuid string) string {
	return fmt.Sprintf("%s%s/%s",
		a.config.Files.BaseURL,
		a.config.Files.Path.CG,
		uuid)
}
