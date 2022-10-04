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

// ISCSIExtent defines model for ISCSIExtent.
// {
//   "id": 218,
//   "name": "c28_disk_md4",
//   "serial": "08002734c651217",
//   "type": "DISK",
//   "path": "zvol/osvcdata/c28_disk_md4",
//   "filesize": "0",
//   "blocksize": 512,
//   "pblocksize": false,
//   "avail_threshold": null,
//   "comment": "",
//   "naa": "0x6589cfc000000f487531b4688113d131",
//   "insecure_tpc": true,
//   "xen": false,
//   "rpm": "SSD",
//   "ro": false,
//   "enabled": true,
//   "vendor": "TrueNAS",
//   "disk": "zvol/osvcdata/c28_disk_md4",
//   "locked": false
// }
type ISCSIExtent struct {
	Id             int     `json:"id"`
	Name           string  `json:"name"`
	Serial         string  `json:"serial"`
	Type           string  `json:"type"`
	Path           string  `json:"path"`
	Filesize       string  `json:"filesize"`
	Blocksize      uint64  `json:"blocksize"`
	PBlocksize     bool    `json:"pblocksize"`
	AvailThreshold *uint64 `json:"avail_threshold"`
	Comment        string  `json:"comment"`
	NAA            string  `json:"naa"`
	InsecureTPC    bool    `json:"insecure_tpc"`
	Xen            bool    `json:"xen"`
	RPM            string  `json:"rpm"`
	RO             bool    `json:"ro"`
	Enabled        bool    `json:"enabled"`
	Vendor         string  `json:"vendor"`
	Disk           string  `json:"disk"`
	Locked         bool    `json:"locked"`
}

type ISCSIExtents []ISCSIExtent

// ISCSIExtentsResponse defines model for ISCSIExtentsResponse.
type ISCSIExtentsResponse = []ISCSIExtent

// GetISCSIExtentsParams defines parameters for GetISCSIExtents.
type GetISCSIExtentsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

type GetISCSIExtentsResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *ISCSIExtentsResponse
}

// Status returns HTTPResponse.Status
func (r GetISCSIExtentsResponse) Status() string {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.Status
	}
	return http.StatusText(0)
}

// StatusCode returns HTTPResponse.StatusCode
func (r GetISCSIExtentsResponse) StatusCode() int {
	if r.HTTPResponse != nil {
		return r.HTTPResponse.StatusCode
	}
	return 0
}

func (c *Client) GetISCSIExtents(ctx context.Context, params *GetISCSIExtentsParams, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetISCSIExtentsRequest(c.Server, params)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetISCSIExtentsRequest generates requests for GetISCSIExtents
func NewGetISCSIExtentsRequest(server string, params *GetISCSIExtentsParams) (*http.Request, error) {
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

// GetISCSIExtentsWithResponse request returning *GetISCSIExtentsResponse
func (c *ClientWithResponses) GetISCSIExtentsWithResponse(ctx context.Context, params *GetISCSIExtentsParams, reqEditors ...RequestEditorFn) (*GetISCSIExtentsResponse, error) {
	rsp, err := c.GetISCSIExtents(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetISCSIExtentsResponse(rsp)
}

// ParseGetISCSIExtentsResponse parses an HTTP response from a GetISCSIExtentsWithResponse call
func ParseGetISCSIExtentsResponse(rsp *http.Response) (*GetISCSIExtentsResponse, error) {
	bodyBytes, err := ioutil.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetISCSIExtentsResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest ISCSIExtentsResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}

func (t ISCSIExtents) GetByName(name string) (ISCSIExtent, bool) {
	for _, e := range t {
		if e.Name == name {

			return e, true
		}
	}
	return ISCSIExtent{}, false
}

func (t ISCSIExtents) GetByPath(s string) (ISCSIExtent, bool) {
	for _, e := range t {
		if e.Path == s {

			return e, true
		}
	}
	return ISCSIExtent{}, false
}
