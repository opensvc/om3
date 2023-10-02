package arrayfreenas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
)

// ISCSITarget defines model for ISCSITarget.
//
//	{
//	 "id": 79,
//	 "name": "iqn.2009-11.com.opensvc.srv:qau20c26n3.storage.target.1",
//	 "alias": null,
//	 "mode": "ISCSI",
//	 "groups": [
//	  {
//	   "portal": 1,
//	   "initiator": 43,
//	   "auth": null,
//	   "authmethod": "NONE"
//	  }
//	 ]
//	},
type ISCSITarget struct {
	Id     int     `json:"id"`
	Name   string  `json:"name"`
	Alias  *string `json:"alias,omitempty"`
	Mode   string
	Groups ISCSITargetGroups
}

type ISCSITargets []ISCSITarget

type ISCSITargetGroups []ISCSITargetGroup

type ISCSITargetGroup struct {
	PortalId    int     `json:"portal"`
	InitiatorId int     `json:"initiator"`
	Auth        *string `json:"auth"`
	AuthMethod  string  `json:"authmethod"`
}

// ISCSITargetsResponse defines model for ISCSITargetsResponse.
type ISCSITargetsResponse = []ISCSITarget

// GetISCSITargetsParams defines parameters for GetISCSITargets.
type GetISCSITargetsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

type GetISCSITargetsResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *ISCSITargetsResponse
}

// Status returns HTTPResponse.Status
func (r GetISCSITargetsResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetISCSITargetsResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) GetISCSITargets(ctx context.Context, params *GetISCSITargetsParams, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetISCSITargetsRequest(c.Server, params)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetISCSITargetsRequest generates requests for GetISCSITargets
func NewGetISCSITargetsRequest(server string, params *GetISCSITargetsParams) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/iscsi/target")
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

// GetISCSITargetsWithResponse request returning *GetISCSITargetsResponse
func (c *ClientWithResponses) GetISCSITargetsWithResponse(ctx context.Context, params *GetISCSITargetsParams, reqEditors ...RequestEditorFn) (*GetISCSITargetsResponse, error) {
	rsp, err := c.GetISCSITargets(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetISCSITargetsResponse(rsp)
}

// ParseGetISCSITargetsResponse parses an HTTP response from a GetISCSITargetsWithResponse call
func ParseGetISCSITargetsResponse(rsp *http.Response) (*GetISCSITargetsResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetISCSITargetsResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest ISCSITargetsResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

func (t ISCSITargets) GetByID(id int) (ISCSITarget, bool) {
	for _, e := range t {
		if e.Id == id {

			return e, true
		}
	}
	return ISCSITarget{}, false
}

func (t ISCSITargets) GetByName(name string) (ISCSITarget, bool) {
	for _, e := range t {
		if e.Name == name {

			return e, true
		}
	}
	return ISCSITarget{}, false
}
