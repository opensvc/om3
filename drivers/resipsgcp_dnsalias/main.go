package resipsgcp_dnsalias

import (
	"context"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

type (
	T struct {
		resource.T
		resource.Restart
		datarecv.DataRecv
	}
)

// New creates a new SGCP NFS filesystem resource driver
func New() resource.Driver {
	return &T{}
}

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}
