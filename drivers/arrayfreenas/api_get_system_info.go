package arrayfreenas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SystemInfo defines model for GetSystemInfo
type SystemInfo struct {
	Version              string     `json:"version"`                // "TrueNAS-13.0-U2"
	Hostname             string     `json:"hostname"`               //  "truenas.vdc.opensvc.com"
	PhysMem              uint64     `json:"physmem"`                // 4241022976
	Model                string     `json:"model"`                  // "Intel(R) Core(TM) i7-10710U CPU @ 1.10GHz"
	Cores                uint       `json:"cores"`                  // 2
	Uptime               string     `json:"uptime"`                 // "4 days, 4:59:17.134670"
	UptimeSeconds        float64    `json:"uptime_seconds"`         // 363557.134669586
	SystemSerial         string     `json:"system_serial"`          // "0",
	SystemProduct        string     `json:"system_product"`         // "VirtualBox"
	SystemProductVersion string     `json:"system_product_version"` // "1.2"
	Timezone             string     `json:"timezone"`               // "Europe/Paris"
	SystemManufacturer   string     `json:"system_manufacturer"`    // "innotek GmbH"
	LoadAvg              [3]float64 `json:"loadavg"`                // [0.32470703125, 0.39111328125, 0.3564453125]
	//  "buildtime": {
	//   "$date": 1661831610000
	//  },
	//  "license": null,
	//  "boottime": {
	//   "$date": 1663040814000
	//  },
	//  "datetime": {
	//   "$date": 1663404371500
	//  },
	//  "birthday": {
	//   "$date": 1658217925615
	//  },
	//  "ecc_memory": null

}

type GetSystemInfoResponse struct {
	Body         []byte
	HTTPResponse *http.Response
	JSON200      *SystemInfo
}

func (c *Client) GetSystemInfo(ctx context.Context, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewGetSystemInfoRequest(c.Server)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewGetSystemInfoRequest generates requests for GetSystemInfo
func NewGetSystemInfoRequest(server string) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/system/info")
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// GetSystemInfoWithResponse request returning *GetSystemInfoResponse
func (c *ClientWithResponses) GetSystemInfoWithResponse(ctx context.Context, reqEditors ...RequestEditorFn) (*GetSystemInfoResponse, error) {
	rsp, err := c.GetSystemInfo(ctx, reqEditors...)
	if err != nil {
		return nil, err
	}
	return ParseGetSystemInfoResponse(rsp)
}

// ParseGetSystemInfoResponse parses an HTTP response from a GetSystemInfoWithResponse call
func ParseGetSystemInfoResponse(rsp *http.Response) (*GetSystemInfoResponse, error) {
	bodyBytes, err := io.ReadAll(rsp.Body)
	defer func() { _ = rsp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	response := &GetSystemInfoResponse{
		Body:         bodyBytes,
		HTTPResponse: rsp,
	}

	switch {
	case rsp.StatusCode == 200:
		var dest SystemInfo
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		response.JSON200 = &dest

	}

	return response, nil
}
