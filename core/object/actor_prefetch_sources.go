package object

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

// imageSourceClient is what the prefetch asks the daemon of the node: where
// the resources of the object are up, and the archive of an image of
// another node, which the daemon relays.
type imageSourceClient interface {
	GetResourcesWithResponse(ctx context.Context, params *api.GetResourcesParams, reqEditors ...api.RequestEditorFn) (*api.GetResourcesResponse, error)
	GetNodeImageSave(ctx context.Context, nodename api.InPathNodeName, params *api.GetNodeImageSaveParams, reqEditors ...api.RequestEditorFn) (*http.Response, error)
}

// newImageSourceClient returns the client of the daemon of the node. An
// archive is sent for as long as its image is large: the client sets no
// timeout, and the context of the prefetch bounds the request. It is
// replaced by the tests.
var newImageSourceClient = func() (imageSourceClient, error) {
	return client.New(client.WithTimeout(0))
}

// imageSources returns the nodes the prefetch of the image of a resource of
// the object may copy the image from, in the order to ask them, by rid.
//
// They are the other nodes of the object, the ones of its nodes keyword, and
// never another: the nodes an image of the object is meant to be on. The
// ones where the resource is up come first: the container runs from the
// image there, which is the case of an image loaded by hand on the node
// where the object was then started. The others follow in the order of the
// keyword, and answer whether they hold the image when asked for it.
//
// Which nodes have the resource up is asked to the daemon. One that does not
// answer changes the order alone: it is also the one relaying the requests
// to the nodes, which then say so, each.
func (t *actor) imageSources(ctx context.Context) func(rid string) []resource.ImageSource {
	none := func(string) []resource.ImageSource { return nil }
	nodes, err := t.Nodes()
	if err != nil {
		t.log.Debugf("prefetch: no node to copy an image from: %s", err)
		return none
	}
	localhost := hostname.Hostname()
	nodes = slices.DeleteFunc(slices.Clone(nodes), func(s string) bool { return s == localhost })
	if len(nodes) == 0 {
		return none
	}
	c, err := newImageSourceClient()
	if err != nil {
		t.log.Debugf("prefetch: no node to copy an image from: %s", err)
		return none
	}
	up, err := t.nodesWithResourceUp(ctx, c)
	if err != nil {
		t.log.Debugf("prefetch: the nodes are asked in the order of the nodes keyword: %s", err)
	}
	return func(rid string) []resource.ImageSource {
		l := make([]resource.ImageSource, 0, len(nodes))
		for _, node := range orderImageSourceNodes(nodes, up[rid]) {
			l = append(l, apiImageSource{client: c, node: node})
		}
		return l
	}
}

// orderImageSourceNodes returns nodes with the ones where the resource is up
// first, each group in the order of nodes.
func orderImageSourceNodes(nodes []string, up map[string]bool) []string {
	l := make([]string, 0, len(nodes))
	for _, want := range []bool{true, false} {
		for _, node := range nodes {
			if up[node] == want {
				l = append(l, node)
			}
		}
	}
	return l
}

// nodesWithResourceUp returns the nodes where each resource of the object is
// up, by rid, as the daemon knows them.
func (t *actor) nodesWithResourceUp(ctx context.Context, c imageSourceClient) (map[string]map[string]bool, error) {
	p := t.path.String()
	resp, err := c.GetResourcesWithResponse(ctx, &api.GetResourcesParams{Path: &p})
	if err != nil {
		return nil, err
	} else if resp.JSON200 == nil {
		return nil, fmt.Errorf("get the resources of %s: %s", p, resp.Status())
	}
	m := make(map[string]map[string]bool)
	for _, item := range resp.JSON200.Items {
		if item.Meta.EncapNode != "" || item.Data.Status == nil || item.Data.Status.Status != status.Up {
			continue
		}
		if m[item.Meta.RID] == nil {
			m[item.Meta.RID] = make(map[string]bool)
		}
		m[item.Meta.RID][item.Meta.Node] = true
	}
	return m, nil
}

// apiImageSource is a node asked for the archive of an image through the
// api of the daemon.
type apiImageSource struct {
	client imageSourceClient
	node   string
}

func (t apiImageSource) Node() string {
	return t.node
}

// Save implements resource.ImageSource, with the image save request of the
// node: the body of the response is the archive, left to be read as it
// comes.
func (t apiImageSource) Save(ctx context.Context, pull resource.Pull) (*resource.ImageArchive, error) {
	params := api.GetNodeImageSaveParams{Engine: pull.Engine, Ref: pull.Image}
	if pull.Store != "" {
		params.Store = &pull.Store
	}
	resp, err := t.client.GetNodeImageSave(ctx, t.node, &params)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return &resource.ImageArchive{
			ID:       resp.Header.Get(api.HeaderImageID),
			IDKind:   resp.Header.Get(api.HeaderImageIDKind),
			Platform: resp.Header.Get(api.HeaderPlatform),
			Body:     resp.Body,
		}, nil
	case http.StatusNotFound:
		// The store of the node does not hold the image, or the node
		// runs an agent that does not know the request.
		_ = resp.Body.Close()
		return nil, resource.ErrImageNotFound
	}
	defer func() { _ = resp.Body.Close() }()
	var problem api.Problem
	if b, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)); err == nil && json.Unmarshal(b, &problem) == nil && problem.Title != "" {
		return nil, fmt.Errorf("%s", problem)
	}
	return nil, fmt.Errorf("%s", resp.Status)
}
