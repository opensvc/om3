package arrayfreenas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// CreateISCSITargetExtentParams defines model for CreateISCSITargetExtentParams.
type CreateISCSITargetExtentParams struct {
	Target int  `json:"target"`
	Extent int  `json:"extent"`
	LunID  *int `json:"lunid"`
}

// CreateISCSITargetExtentJSONRequestBody defines body for CreateISCSITargetExtent for application/json ContentType.
type CreateISCSITargetExtentJSONRequestBody = CreateISCSITargetExtentJSONBody

// CreateISCSITargetExtentJSONBody defines parameters for CreateISCSITargetExtent.
type CreateISCSITargetExtentJSONBody = CreateISCSITargetExtentParams

type CreateISCSITargetExtentResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *ISCSITargetExtent
}

// Status returns HTTPResponse.Status
func (r CreateISCSITargetExtentResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r CreateISCSITargetExtentResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) CreateISCSITargetExtent(ctx context.Context, body CreateISCSITargetExtentJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateISCSITargetExtentRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// CreateISCSITargetExtentWithBodyWithResponse request with arbitrary body returning *CreateISCSITargetExtentResponse
func (c *ClientWithResponses) CreateISCSITargetExtentWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateISCSITargetExtentResponse, error) {
	rsp, err := c.CreateISCSITargetExtentWithBody(ctx, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateISCSITargetExtentResponse(rsp)
}

func (c *ClientWithResponses) CreateISCSITargetExtentWithResponse(ctx context.Context, body CreateISCSITargetExtentJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateISCSITargetExtentResponse, error) {
	rsp, err := c.CreateISCSITargetExtent(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateISCSITargetExtentResponse(rsp)
}

// ParseCreateISCSITargetExtentResponse parses an HTTP response from a CreateISCSITargetExtentWithResponse call
func ParseCreateISCSITargetExtentResponse(rsp *http.Response) (*CreateISCSITargetExtentResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateISCSITargetExtentResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest ISCSITargetExtent
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

// NewCreateISCSITargetExtentRequest calls the generic CreateISCSITargetExtent builder with application/json body
func NewCreateISCSITargetExtentRequest(server string, body CreateISCSITargetExtentJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewCreateISCSITargetExtentRequestWithBody(server, "application/json", bodyReader)
}

// NewCreateISCSITargetExtentRequestWithBody generates requests for CreateISCSITargetExtent with any type of body
func NewCreateISCSITargetExtentRequestWithBody(server string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/iscsi/targetextent")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

func (c *Client) CreateISCSITargetExtentWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateISCSITargetExtentRequestWithBody(c.Server, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
