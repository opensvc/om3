package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/opensvc/om3/v3/daemon/api"
)

type GetClusterStatus struct {
	client    api.ClientInterface
	namespace *string
	selector  *string
	relatives *bool
}

func (t *GetClusterStatus) SetNamespace(s string) *GetClusterStatus {
	t.namespace = &s
	return t
}

func (t *GetClusterStatus) SetSelector(s string) *GetClusterStatus {
	t.selector = &s
	return t
}

func NewGetClusterStatus(t api.ClientInterface) *GetClusterStatus {
	options := &GetClusterStatus{
		client: t,
	}
	return options
}

// Get fetches the daemon status structure from the agent api
func (t GetClusterStatus) Get() ([]byte, error) {
	params := api.GetClusterStatusParams{
		Namespace: t.namespace,
		Selector:  t.selector,
	}
	resp, err := t.client.GetClusterStatus(context.Background(), &params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected get daemon status code %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
