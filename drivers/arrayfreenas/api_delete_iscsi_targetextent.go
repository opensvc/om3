package arrayfreenas

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
)

type DeleteISCSITargetExtentResponse struct {
	Body         []byte
	HTTPResponse *http.Response
}

// Status returns HTTPResponse.Status
func (r DeleteISCSITargetExtentResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r DeleteISCSITargetExtentResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ParseDeleteISCSITargetExtentResponse parses an HTTP response from a DeleteISCSITargetExtentWithResponse call
func ParseDeleteISCSITargetExtentResponse(rsp *http.Response) (*DeleteISCSITargetExtentResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &DeleteISCSITargetExtentResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	return response, nil
}

// DeleteISCSITargetExtentWithResponse request returning *DeleteISCSITargetExtentResponse
func (c *ClientWithResponses) DeleteISCSITargetExtentWithResponse(ctx context.Context, id string, reqEditors ...RequestEditorFn) (*DeleteISCSITargetExtentResponse, error) {
	rsp, err := c.DeleteISCSITargetExtent(ctx, id, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseDeleteISCSITargetExtentResponse(rsp)
}

// NewDeleteISCSITargetExtentRequest generates requests for DeleteISCSITargetExtent
func NewDeleteISCSITargetExtentRequest(server string, id string) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "id", runtime.ParamLocationPath, id)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/iscsi/targetextent/id/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("DELETE", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *Client) DeleteISCSITargetExtent(ctx context.Context, id string, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewDeleteISCSITargetExtentRequest(c.Server, id)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
