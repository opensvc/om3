package arrayfreenas

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
)

// Pool defines model for Pool.
type Pool struct {
	EncryptkeyPath *string `json:"encryptkey_path,omitempty"`
	Guid           *string `json:"guid,omitempty"`
	Healthy        *bool   `json:"healthy,omitempty"`
	Id             int     `json:"id"`
	IsDecrypted    *bool   `json:"is_decrypted,omitempty"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	Status         *string `json:"status,omitempty"`
}

// PoolsResponse defines model for PoolsResponse.
type PoolsResponse = []Pool

// GetPoolsParams defines parameters for GetPools.
type GetPoolsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

type GetPoolsResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *PoolsResponse
}

// Status returns HTTPResponse.Status
func (r GetPoolsResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetPoolsResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) GetPools(ctx context.Context, params *GetPoolsParams, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetPoolsRequest(c.Server, params)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetPoolsRequest generates requests for GetPools
func NewGetPoolsRequest(server string, params *GetPoolsParams) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/pool")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	queryValues := queryURL.Query()

	if params.Limit != nil {

		if queryFrag, err := runtime.StyleParamWithLocation("form", true, "limit", runtime.ParamLocationQuery, *params.Limit); err != nil {
			return nil, err
		} else if parsed, err := url.ParseQuery(queryFrag); err != nil {
			return nil, err
		} else {
			for k, v := range parsed {
				for _, v2 := range v {
					queryValues.Add(k, v2)
				}
			}
		}

	}

	if params.Offset != nil {

		if queryFrag, err := runtime.StyleParamWithLocation("form", true, "offset", runtime.ParamLocationQuery, *params.Offset); err != nil {
			return nil, err
		} else if parsed, err := url.ParseQuery(queryFrag); err != nil {
			return nil, err
		} else {
			for k, v := range parsed {
				for _, v2 := range v {
					queryValues.Add(k, v2)
				}
			}
		}

	}

	if params.Count != nil {

		if queryFrag, err := runtime.StyleParamWithLocation("form", true, "count", runtime.ParamLocationQuery, *params.Count); err != nil {
			return nil, err
		} else if parsed, err := url.ParseQuery(queryFrag); err != nil {
			return nil, err
		} else {
			for k, v := range parsed {
				for _, v2 := range v {
					queryValues.Add(k, v2)
				}
			}
		}

	}

	if params.Sort != nil {

		if queryFrag, err := runtime.StyleParamWithLocation("form", true, "sort", runtime.ParamLocationQuery, *params.Sort); err != nil {
			return nil, err
		} else if parsed, err := url.ParseQuery(queryFrag); err != nil {
			return nil, err
		} else {
			for k, v := range parsed {
				for _, v2 := range v {
					queryValues.Add(k, v2)
				}
			}
		}

	}

	queryURL.RawQuery = queryValues.Encode()

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// GetPoolsWithResponse request returning *GetPoolsResponse
func (c *ClientWithResponses) GetPoolsWithResponse(ctx context.Context, params *GetPoolsParams, reqEditors ...RequestEditorFn) (*GetPoolsResponse, error) {
	rsp, err := c.GetPools(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetPoolsResponse(rsp)
}

// ParseGetPoolsResponse parses an HTTP response from a GetPoolsWithResponse call
func ParseGetPoolsResponse(rsp *http.Response) (*GetPoolsResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetPoolsResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	//case strings.Contains(rsp.Header.Get("Content-Type"), "application/json") && rsp.StatusCode == 200:
	case rsp.StatusCode == 200:
		var dest PoolsResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}
