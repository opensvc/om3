package client

import (
	"fmt"
	"net"
	"net/http"
)

type (
	// JSONRPC is the agent JSON RPC api struct
	JSONRPC struct {
		URL string
	}
)

const (
	// JSONRPCScheme is the JSONRPC protocol scheme prefix in URL
	JSONRPCScheme string = "raw://"

	// JSONRPCUDSPath is the default location of the JSONRPC Unix Domain Socket
	JSONRPCUDSPath string = "/opt/opensvc/var/lsnr/lsnr.sock"
)

// Get implements the Get interface method for the JSONRPC api
func (t JSONRPC) Get(req string) (*http.Response, error) {
	conn, err := net.Dial("unix", JSONRPCUDSPath)

	if err != nil {
		return nil, err
	}
	jsonStr := `{"method": "GET", "action": "` + req + `"}`
	_, err = fmt.Fprintf(conn, jsonStr)
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
