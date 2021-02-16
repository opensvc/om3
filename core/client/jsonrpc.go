package client

import (
	"encoding/json"
	"net"
	"net/http"
)

type (
	// JSONRPC is the agent JSON RPC api struct
	JSONRPC struct {
		URL string
	}

	jsonrpcRequest struct {
		Method  string                 `json:"method"`
		Action  string                 `json:"action"`
		Node    string                 `json:"node"`
		Options map[string]interface{} `json:"options"`
	}
)

const (
	// JSONRPCScheme is the JSONRPC protocol scheme prefix in URL
	JSONRPCScheme string = "raw://"

	// JSONRPCUDSPath is the default location of the JSONRPC Unix Domain Socket
	JSONRPCUDSPath string = "/opt/opensvc/var/lsnr/lsnr.sock"
)

func newJSONRPCRequest(method string, action string, node string, opts map[string]interface{}) *jsonrpcRequest {
	if opts == nil {
		opts = make(map[string]interface{})
	}
	return &jsonrpcRequest{
		Method:  method,
		Action:  action,
		Node:    node,
		Options: opts,
	}
}

// Get implements the Get interface method for the JSONRPC api
func (t JSONRPC) Get(path string, opts RequestOptions) (*http.Response, error) {
	conn, err := net.Dial("unix", JSONRPCUDSPath)

	if err != nil {
		return nil, err
	}
	req := newJSONRPCRequest("GET", path, opts.Node, nil)
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	conn.Write(b)
	conn.Write([]byte("\x00"))
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp := &http.Response{
		Body: conn,
	}
	return resp, nil
}

func newJSONRPC(c Config) JSONRPC {
	return JSONRPC{}
}
