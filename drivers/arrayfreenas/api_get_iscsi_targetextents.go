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

// ISCSITargetExtent defines model for ISCSITargetExtent.
//     {
//        "id": 1463,
//        "lunid": 42,
//        "extent": 211,
//        "target": 76
//    }
type ISCSITargetExtent struct {
	Id       int `json:"id"`
	LunId    int `json:"lunid"`
	ExtentId int `json:"extent"`
	TargetId int `json:"target"`
}

// ISCSITargetExtentsResponse defines model for ISCSITargetExtentsResponse.
type ISCSITargetExtentsResponse = []ISCSITargetExtent

// GetISCSITargetExtentsParams defines parameters for GetISCSITargetExtents.
type GetISCSITargetExtentsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

type GetISCSITargetExtentsResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *ISCSITargetExtentsResponse
}

// Status returns HTTPResponse.Status
func (r GetISCSITargetExtentsResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetISCSITargetExtentsResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) GetISCSITargetExtents(ctx context.Context, params *GetISCSITargetExtentsParams, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetISCSITargetExtentsRequest(c.Server, params)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetISCSITargetExtentsRequest generates requests for GetISCSITargetExtents
func NewGetISCSITargetExtentsRequest(server string, params *GetISCSITargetExtentsParams) (*http.Request, error) {
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

// GetISCSITargetExtentsWithResponse request returning *GetISCSITargetExtentsResponse
func (c *ClientWithResponses) GetISCSITargetExtentsWithResponse(ctx context.Context, params *GetISCSITargetExtentsParams, reqEditors ...RequestEditorFn) (*GetISCSITargetExtentsResponse, error) {
	rsp, err := c.GetISCSITargetExtents(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetISCSITargetExtentsResponse(rsp)
}

// ParseGetISCSITargetExtentsResponse parses an HTTP response from a GetISCSITargetExtentsWithResponse call
func ParseGetISCSITargetExtentsResponse(rsp *http.Response) (*GetISCSITargetExtentsResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetISCSITargetExtentsResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest ISCSITargetExtentsResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}
