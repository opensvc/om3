package api

import (
	"context"
	"io"
	"net/http"

	"github.com/pkg/errors"

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

func (t *GetDaemonStatus) SetRelatives(s bool) *GetDaemonStatus {
	t.relatives = &s
	return t
}

func NewGetDaemonStatus(t api.ClientInterface) *GetDaemonStatus {
	options := &GetDaemonStatus{
		client: t,
	}
	return options
}

// Do fetches the daemon status structure from the agent api
func (t GetDaemonStatus) Get() ([]byte, error) {
	params := api.GetDaemonStatusParams{
		Namespace: t.namespace,
		Selector:  t.selector,
		Relatives: t.relatives,
	}
	resp, err := t.client.GetDaemonStatus(context.Background(), &params)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected get daemon status code %s", resp.Status)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
