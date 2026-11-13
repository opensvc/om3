package commoncmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// ObjectCollectorTagShow returns the work of "<path> collector tag show":
// the names of the tags attached to the object, or all their data with
// verbose.
func ObjectCollectorTagShow(verbose bool) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		resp, err := c.GetObjectCollectorTagAttachmentsWithResponse(ctx, p.Namespace, p.Kind, p.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return tagAttachments(*resp.JSON200, verbose), nil
	}
}

// ObjectCollectorTagAttach returns the work of "<path> collector tag
// attach": the answer of the collector.
func ObjectCollectorTagAttach(name string, attachData *string) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		if name == "" {
			return nil, errors.New("the tag name must not be empty")
		}
		resp, err := c.PostObjectCollectorTagAttachmentWithResponse(ctx, p.Namespace, p.Kind, p.Name, name, api.CollectorTagAttach{AttachData: attachData})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return resp.JSON200.Info, nil
	}
}

// ObjectCollectorTagDetach returns the work of "<path> collector tag
// detach": the answer of the collector.
func ObjectCollectorTagDetach(name string) ObjectAPIFunc {
	return func(ctx context.Context, c *client.T, p naming.Path) (any, error) {
		if name == "" {
			return nil, errors.New("the tag name must not be empty")
		}
		resp, err := c.DeleteObjectCollectorTagAttachmentWithResponse(ctx, p.Namespace, p.Kind, p.Name, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if resp.JSON200 == nil {
			return nil, objectResponseError(p, resp, resp.Body)
		}
		return resp.JSON200.Info, nil
	}
}
