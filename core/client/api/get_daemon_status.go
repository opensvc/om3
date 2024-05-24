package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/opensvc/om3/daemon/api"
)

type GetDaemonStatus struct {
	client    api.ClientInterface
	namespace *string
	selector  *string
	relatives *bool
}

func (t *GetDaemonStatus) SetNamespace(s string) *GetDaemonStatus {
	t.namespace = &s
	return t
}

func (t *GetDaemonStatus) SetSelector(s string) *GetDaemonStatus {
	t.selector = &s
	return t
}

func NewGetDaemonStatus(t api.ClientInterface) *GetDaemonStatus {
	options := &GetDaemonStatus{
		client: t,
	}
	return options
}

// Get fetches the daemon status structure from the agent api
func (t GetDaemonStatus) Get() ([]byte, error) {
	params := api.GetDaemonStatusParams{
		Namespace: t.namespace,
		Selector:  t.selector,
	}
	resp, err := t.client.GetDaemonStatus(context.Background(), &params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected get daemon status code %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
