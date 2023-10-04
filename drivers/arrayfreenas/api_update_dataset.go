package arrayfreenas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
)

// UpdateDatasetParams defines model for UpdateDatasetParams.
type UpdateDatasetParams struct {
	Aclmode        *string `json:"aclmode,omitempty"`
	Atime          *string `json:"atime,omitempty"`
	Comments       *string `json:"comments,omitempty"`
	Compression    *string `json:"compression,omitempty"`
	Copies         *int    `json:"copies,omitempty"`
	Deduplication  *string `json:"deduplication,omitempty"`
	Exec           *string `json:"exec,omitempty"`
	ForceSize      *bool   `json:"force_size,omitempty"`
	Quota          *int64  `json:"quota,omitempty"`
	Readonly       *string `json:"readonly,omitempty"`
	Recordsize     *string `json:"recordsize,omitempty"`
	Refquota       *int64  `json:"refquota,omitempty"`
	Refreservation *int64  `json:"refreservation,omitempty"`
	Snapdir        *string `json:"snapdir,omitempty"`
	Sync           *string `json:"sync,omitempty"`
	Volsize        *int64  `json:"volsize,omitempty"`
}

// UpdateDatasetJSONRequestBody defines body for UpdateDataset for application/json ContentType.
type UpdateDatasetJSONRequestBody = UpdateDatasetJSONBody

// UpdateDatasetJSONBody defines parameters for UpdateDataset.
type UpdateDatasetJSONBody = UpdateDatasetParams

type UpdateDatasetResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *Dataset
}

// Status returns HTTPResponse.Status
func (r UpdateDatasetResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r UpdateDatasetResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) UpdateDatasetWithBody(ctx context.Context, id string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewUpdateDatasetRequestWithBody(c.Server, id, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) UpdateDataset(ctx context.Context, id string, body UpdateDatasetJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewUpdateDatasetRequest(c.Server, id, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewUpdateDatasetRequest calls the generic UpdateDataset builder with application/json body
func NewUpdateDatasetRequest(server string, id string, body UpdateDatasetJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewUpdateDatasetRequestWithBody(server, id, "application/json", bodyReader)
}

// NewUpdateDatasetRequestWithBody generates requests for UpdateDataset with any type of body
func NewUpdateDatasetRequestWithBody(server string, id string, contentType string, body io.Reader) (*http.Request, error) {
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

	operationPath := fmt.Sprintf("/pool/dataset/id/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// UpdateDatasetWithBodyWithResponse request with arbitrary body returning *UpdateDatasetResponse
func (c *ClientWithResponses) UpdateDatasetWithBodyWithResponse(ctx context.Context, id string, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*UpdateDatasetResponse, error) {
	rsp, err := c.UpdateDatasetWithBody(ctx, id, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseUpdateDatasetResponse(rsp)
}

// ParseUpdateDatasetResponse parses an HTTP response from a UpdateDatasetWithResponse call
func ParseUpdateDatasetResponse(rsp *http.Response) (*UpdateDatasetResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &UpdateDatasetResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest Dataset
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

func (c *ClientWithResponses) UpdateDatasetWithResponse(ctx context.Context, id string, body UpdateDatasetJSONRequestBody, reqEditors ...RequestEditorFn) (*UpdateDatasetResponse, error) {
	rsp, err := c.UpdateDataset(ctx, id, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseUpdateDatasetResponse(rsp)
}
