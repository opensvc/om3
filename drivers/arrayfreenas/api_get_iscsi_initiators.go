package arrayfreenas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/oapi-codegen/runtime"
)

// ISCSIInitiator defines model for ISCSIInitiator.
//
//	{
//	    "id": 40,
//	    "initiators": [
//	        "iqn.2009-11.com.opensvc.srv:qau22c13n3.storage.initiator"
//	    ],
//	    "auth_network": [],
//	    "comment": ""
//	}
type ISCSIInitiator struct {
	Id         int      `json:"id"`
	Initiators []string `json:"initiators"`
	Comment    string   `json:"comment"`
}

type ISCSIInitiators []ISCSIInitiator

// ISCSIInitiatorsResponse defines model for ISCSIInitiatorsResponse.
type ISCSIInitiatorsResponse = []ISCSIInitiator

// GetISCSIInitiatorsParams defines parameters for GetISCSIInitiators.
type GetISCSIInitiatorsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

type GetISCSIInitiatorsResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *ISCSIInitiatorsResponse
}

// Status returns HTTPResponse.Status
func (r GetISCSIInitiatorsResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetISCSIInitiatorsResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) GetISCSIInitiators(ctx context.Context, params *GetISCSIInitiatorsParams, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetISCSIInitiatorsRequest(c.Server, params)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetISCSIInitiatorsRequest generates requests for GetISCSIInitiators
func NewGetISCSIInitiatorsRequest(server string, params *GetISCSIInitiatorsParams) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/iscsi/initiator")
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

// GetISCSIInitiatorsWithResponse request returning *GetISCSIInitiatorsResponse
func (c *ClientWithResponses) GetISCSIInitiatorsWithResponse(ctx context.Context, params *GetISCSIInitiatorsParams, reqEditors ...RequestEditorFn) (*GetISCSIInitiatorsResponse, error) {
	rsp, err := c.GetISCSIInitiators(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetISCSIInitiatorsResponse(rsp)
}

// ParseGetISCSIInitiatorsResponse parses an HTTP response from a GetISCSIInitiatorsWithResponse call
func ParseGetISCSIInitiatorsResponse(rsp *http.Response) (*GetISCSIInitiatorsResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetISCSIInitiatorsResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest ISCSIInitiatorsResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

func (t ISCSIInitiators) GetByID(id int) (ISCSIInitiator, bool) {
	for _, e := range t {
		if e.Id == id {
			return e, true
		}
	}
	return ISCSIInitiator{}, false
}
