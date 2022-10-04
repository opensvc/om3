package arrayfreenas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

// CreateISCSIExtentParams defines model for CreateISCSIExtentParams.
type CreateISCSIExtentParams struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	InsecureTPC bool   `json:"insecure_tpc"`
	Blocksize   int    `json:"blocksize"`
	Disk        string `json:"disk"`
}

// CreateISCSIExtentJSONRequestBody defines body for CreateISCSIExtent for application/json ContentType.
type CreateISCSIExtentJSONRequestBody = CreateISCSIExtentJSONBody

// CreateISCSIExtentJSONBody defines parameters for CreateISCSIExtent.
type CreateISCSIExtentJSONBody = CreateISCSIExtentParams

type CreateISCSIExtentResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *ISCSIExtent
}

// Status returns HTTPResponse.Status
func (r CreateISCSIExtentResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r CreateISCSIExtentResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) CreateISCSIExtent(ctx context.Context, body CreateISCSIExtentJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateISCSIExtentRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// CreateISCSIExtentWithBodyWithResponse request with arbitrary body returning *CreateISCSIExtentResponse
func (c *ClientWithResponses) CreateISCSIExtentWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateISCSIExtentResponse, error) {
	rsp, err := c.CreateISCSIExtentWithBody(ctx, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateISCSIExtentResponse(rsp)
}

func (c *ClientWithResponses) CreateISCSIExtentWithResponse(ctx context.Context, body CreateISCSIExtentJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateISCSIExtentResponse, error) {
	rsp, err := c.CreateISCSIExtent(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateISCSIExtentResponse(rsp)
}

// ParseCreateISCSIExtentResponse parses an HTTP response from a CreateISCSIExtentWithResponse call
func ParseCreateISCSIExtentResponse(rsp *http.Response) (*CreateISCSIExtentResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateISCSIExtentResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest ISCSIExtent
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

// NewCreateISCSIExtentRequest calls the generic CreateISCSIExtent builder with application/json body
func NewCreateISCSIExtentRequest(server string, body CreateISCSIExtentJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewCreateISCSIExtentRequestWithBody(server, "application/json", bodyReader)
}

// NewCreateISCSIExtentRequestWithBody generates requests for CreateISCSIExtent with any type of body
func NewCreateISCSIExtentRequestWithBody(server string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/iscsi/extent")
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

func (c *Client) CreateISCSIExtentWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateISCSIExtentRequestWithBody(c.Server, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
