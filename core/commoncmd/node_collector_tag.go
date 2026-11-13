package commoncmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

// NodeCollectorTagShowRemote returns the remote function of "node collector
// tag show": the names of the tags attached to the node, or all their data
// with verbose.
func NodeCollectorTagShowRemote(verbose bool) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		return nodeCollectorTagShow(ctx, c, nodename, verbose)
	}
}

func nodeCollectorTagShow(ctx context.Context, c *client.T, nodename string, verbose bool) (any, error) {
	resp, err := c.GetNodeCollectorTagAttachmentsWithResponse(ctx, nodename)
	if err != nil {
		return nil, fmt.Errorf("node %s: %w", nodename, err)
	}
	if resp.JSON200 == nil {
		return nil, nodeResponseError(nodename, resp, resp.Body)
	}
	return tagAttachments(*resp.JSON200, verbose), nil
}

// NodeCollectorTagAttachRemote returns the remote function of "node collector
// tag attach": the answer of the collector.
func NodeCollectorTagAttachRemote(name string, attachData *string) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		if name == "" {
			return nil, errors.New("the tag name must not be empty")
		}
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		body := api.CollectorTagAttach{AttachData: attachData}
		resp, err := c.PostNodeCollectorTagAttachmentWithResponse(ctx, nodename, name, body)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return resp.JSON200.Info, nil
	}
}

// NodeCollectorTagDetachRemote returns the remote function of "node collector
// tag detach": the answer of the collector.
func NodeCollectorTagDetachRemote(name string) func(context.Context, string) (any, error) {
	return func(ctx context.Context, nodename string) (any, error) {
		if name == "" {
			return nil, errors.New("the tag name must not be empty")
		}
		c, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, err
		}
		resp, err := c.DeleteNodeCollectorTagAttachmentWithResponse(ctx, nodename, name)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, nodeResponseError(nodename, resp, resp.Body)
		}
		return resp.JSON200.Info, nil
	}
}

// CollectorTagList lists the tags of the collector, as it answers them to the
// node serving the request.
func CollectorTagList(opts OptsGlobal) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	names, err := collectorTagNames(context.Background(), c)
	if err != nil {
		return err
	}
	return output.Renderer{
		Output:   opts.Output,
		Sort:     opts.Sort,
		Color:    opts.Color,
		Data:     names,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			s := ""
			for _, name := range names {
				s += name + "\n"
			}
			return s
		},
	}.Print()
}

func collectorTagNames(ctx context.Context, c *client.T) ([]string, error) {
	resp, err := c.GetClusterCollectorTagsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiResponseError(resp, resp.Body)
	}
	return []string(*resp.JSON200), nil
}

// CollectorTagCreate creates a tag on the collector, as the node serving the
// request, and prints the answer of the collector.
func CollectorTagCreate(name string, data, exclude *string) error {
	if name == "" {
		return errors.New("the tag name must not be empty")
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	info, err := collectorTagCreate(context.Background(), c, name, data, exclude)
	if err != nil {
		return err
	}
	fmt.Println(info)
	return nil
}

func collectorTagCreate(ctx context.Context, c *client.T, name string, data, exclude *string) (string, error) {
	resp, err := c.PostClusterCollectorTagWithResponse(ctx, api.CollectorTagCreate{Name: name, Data: data, Exclude: exclude})
	if err != nil {
		return "", err
	}
	if resp.JSON200 == nil {
		return "", apiResponseError(resp, resp.Body)
	}
	return resp.JSON200.Info, nil
}

// tagAttachments returns the attachments the api answers as the local
// command returns them: their names, or all their data with verbose.
func tagAttachments(l api.CollectorTagAttachments, verbose bool) any {
	r := make(collector.TagAttachmentList, len(l))
	for i, e := range l {
		r[i] = collector.TagAttachment{TagName: e.TagName, TagData: e.TagData, TagAttachData: e.TagAttachData}
	}
	if verbose {
		return r
	}
	names := make([]string, len(r))
	for i, e := range r {
		names[i] = e.TagName
	}
	return names
}
