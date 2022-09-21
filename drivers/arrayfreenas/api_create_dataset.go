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
	"strings"
)

// CreateDatasetParams defines model for CreateDatasetParams.
type CreateDatasetParams struct {
	Aclmode           *string                               `json:"aclmode,omitempty"`
	Atime             *string                               `json:"atime,omitempty"`
	Casesensitivity   *string                               `json:"casesensitivity,omitempty"`
	Comments          *string                               `json:"comments,omitempty"`
	Compression       *string                               `json:"compression,omitempty"`
	Copies            *int                                  `json:"copies,omitempty"`
	Deduplication     *string                               `json:"deduplication,omitempty"`
	Encryption        *bool                                 `json:"encryption,omitempty"`
	EncryptionOptions *CreateDatasetParamsEncryptionOptions `json:"encryption_options,omitempty"`
	Exec              *string                               `json:"exec,omitempty"`
	ForceSize         *bool                                 `json:"force_size,omitempty"`
	InheritEncryption *bool                                 `json:"inherit_encryption,omitempty"`
	Sparse            *bool                                 `json:"sparse,omitempty"`
	Name              string                                `json:"name"`
	Quota             *int64                                `json:"quota,omitempty"`
	QuotaCritical     *int64                                `json:"quota_critical,omitempty"`
	QuotaWarning      *int64                                `json:"quota_warning,omitempty"`
	Readonly          *string                               `json:"readonly,omitempty"`
	Recordsize        *string                               `json:"recordsize,omitempty"`
	Refquota          *int64                                `json:"refquota,omitempty"`
	RefquotaCritical  *int64                                `json:"refquota_critical,omitempty"`
	RefquotaWarning   *int64                                `json:"refquota_warning,omitempty"`
	Refreservation    *int64                                `json:"refreservation,omitempty"`
	Reservation       *int64                                `json:"reservation,omitempty"`
	ShareType         *string                               `json:"share_type,omitempty"`
	Snapdir           *string                               `json:"snapdir,omitempty"`
	Sync              *string                               `json:"sync,omitempty"`
	Type              *string                               `json:"type,omitempty"`
	Volblocksize      *string                               `json:"volblocksize,omitempty"`
	Volsize           *int64                                `json:"volsize,omitempty"`
}

// CreateDatasetJSONRequestBody defines body for CreateDataset for application/json ContentType.
type CreateDatasetJSONRequestBody = CreateDatasetJSONBody

// CreateDatasetJSONBody defines parameters for CreateDataset.
type CreateDatasetJSONBody = CreateDatasetParams

// CreateDatasetParamsEncryptionOptions defines model for CreateDatasetParams_encryption_options.
type CreateDatasetParamsEncryptionOptions struct {
	Algorithm   *string `json:"algorithm,omitempty"`
	GenerateKey *bool   `json:"generate_key,omitempty"`
	Key         *string `json:"key,omitempty"`
	Passphrase  *string `json:"passphrase,omitempty"`
}

type CreateDatasetResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *Dataset
}

// Status returns HTTPResponse.Status
func (r CreateDatasetResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r CreateDatasetResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) CreateDataset(ctx context.Context, body CreateDatasetJSONRequestBody, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateDatasetRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// CreateDatasetWithBodyWithResponse request with arbitrary body returning *CreateDatasetResponse
func (c *ClientWithResponses) CreateDatasetWithBodyWithResponse(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*CreateDatasetResponse, error) {
	rsp, err := c.CreateDatasetWithBody(ctx, contentType, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateDatasetResponse(rsp)
}

func (c *ClientWithResponses) CreateDatasetWithResponse(ctx context.Context, body CreateDatasetJSONRequestBody, reqEditors ...RequestEditorFn) (*CreateDatasetResponse, error) {
	rsp, err := c.CreateDataset(ctx, body, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseCreateDatasetResponse(rsp)
}

// ParseCreateDatasetResponse parses an HTTP response from a CreateDatasetWithResponse call
func ParseCreateDatasetResponse(rsp *http.Response) (*CreateDatasetResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &CreateDatasetResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 200:
		var dest Dataset
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

// NewCreateDatasetRequest calls the generic CreateDataset builder with application/json body
func NewCreateDatasetRequest(server string, body CreateDatasetJSONRequestBody) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewCreateDatasetRequestWithBody(server, "application/json", bodyReader)
}

// NewCreateDatasetRequestWithBody generates requests for CreateDataset with any type of body
func NewCreateDatasetRequestWithBody(server string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/pool/dataset")
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

func (c *Client) CreateDatasetWithBody(ctx context.Context, contentType string, body io.Reader, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateDatasetRequestWithBody(c.Server, contentType, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}
