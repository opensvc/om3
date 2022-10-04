package arrayfreenas

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
)

type DeleteISCSIExtentResponse struct {
	Body         []byte
	HTTPResponse *http.Response
}

// Status returns HTTPResponse.Status
func (r DeleteISCSIExtentResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r DeleteISCSIExtentResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

// ParseDeleteISCSIExtentResponse parses an HTTP response from a DeleteISCSIExtentWithResponse call
func ParseDeleteISCSIExtentResponse(rsp *http.Response) (*DeleteISCSIExtentResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &DeleteISCSIExtentResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	return response, nil
}

// DeleteISCSIExtentWithResponse request returning *DeleteISCSIExtentResponse
func (c *ClientWithResponses) DeleteISCSIExtentWithResponse(ctx context.Context, id string, reqEditors ...RequestEditorFn) (*DeleteISCSIExtentResponse, error) {
	rsp, err := c.DeleteISCSIExtent(ctx, id, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseDeleteISCSIExtentResponse(rsp)
}

// NewDeleteISCSIExtentRequest generates requests for DeleteISCSIExtent
func NewDeleteISCSIExtentRequest(server string, id string) (*http.Request, error) {
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

	operationPath := fmt.Sprintf("/iscsi/extent/id/%s", pathParam0)
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

func (c *Client) DeleteISCSIExtent(ctx context.Context, id string, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewDeleteISCSIExtentRequest(c.Server, id)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
